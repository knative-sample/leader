package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/knative-sample/leader/pkg/signals"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var (
	appCommand      *exec.Cmd
	kubecofig       string
	appCommandStr   string
	leaderID        string
	leaderNamespace string
)

func parseArgs() {
	flag.StringVar(&kubecofig, "kubecofig", "", "kubeconfig path")
	flag.StringVar(&appCommandStr, "app-command", "", "app command")
	flag.StringVar(&leaderID, "leader-id", "", "leader id")
	flag.StringVar(&leaderNamespace, "leader-namespace", "", "leader namespace")
	flag.Parse()
}

func main() {
	parseArgs()
	leaderNS := getFirstValue([]string{
		leaderNamespace,
		os.Getenv("LEADER_NAMESPACE"),
		"default",
	})

	if appCommandStr == "" {
		log.Fatalf("-app-command is empty")
	}
	if leaderID == "" {
		log.Fatalf("-leader-id is empty")
	}
	leaderName := fmt.Sprintf("%s-lock", leaderID)

	cliConfig, err := GetConfig("", kubecofig)
	if err != nil {
		log.Fatalf("new kube client error: %v", err)
	}

	kubecli, err := kubernetes.NewForConfig(cliConfig)
	if err != nil {
		log.Fatalf("NewForConfig error: %v", err)
	}

	log.Println("run with leader-elect")

	id, err := os.Hostname()
	if err != nil {
		log.Fatalf("get hostname error: %v", err)
	}

	id = id + "_" + uuid.New().String()
	rl, err := resourcelock.New("endpoints", // support endpoints and configmaps
		leaderNS,
		leaderName,
		kubecli.CoreV1(),
		kubecli.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
	if err != nil {
		log.Fatalf("create ResourceLock error: %v", err)
	}

	ctx := signals.NewContext()

	go func(ctx context.Context) {
		<-ctx.Done()
		log.Printf("term signal received!!")
		if appCommand != nil {
			if err := appCommand.Process.Kill(); err != nil {
				log.Fatalf("term signal received, kill app command error:%s", err)
			} else {
				log.Printf("term signal received, kill app process")
			}
		}

		os.Exit(0)
	}(ctx)

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Println("you are the leader")
				run(ctx)
			},
			OnStoppedLeading: func() {
				if appCommand != nil {
					if err := appCommand.Process.Kill(); err != nil {
						log.Fatalf("kill app command error:%s", err)
					}
				}
				log.Fatalf("leaderelection lost")
			},
		},
		Name: leaderName,
	})
}

func getFirstValue(vals []string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}

	return ""
}

func run(ctx context.Context) {
	for {
		log.Println("I'm working, start app process")
		start(ctx)
		time.Sleep(time.Second * 1)
	}
}

func start(ctx context.Context) {
	appCommand = exec.Command(appCommandStr)
	stdout, err := appCommand.StdoutPipe()
	if err != nil {
		log.Fatalf("get stdout error:%s", err)
	}
	stderr, err := appCommand.StderrPipe()
	if err != nil {
		log.Fatalf("get stderr error:%s", err)
	}

	err = appCommand.Start()
	if err != nil {
		log.Fatalf("start command error:%s", err)
	}
	defer appCommand.Wait() //Doesn't block
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
}

// GetConfig returns a rest.Config to be used for kubernetes client creation.
// It does so in the following order:
//   1. Use the passed kubeconfig/masterURL.
//   2. Fallback to the KUBECONFIG environment variable.
//   3. Fallback to in-cluster config.
//   4. Fallback to the ~/.kube/config.
func GetConfig(masterURL, kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")
}
