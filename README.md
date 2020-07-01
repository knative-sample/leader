# leader
kubernes 集群通用的抢锁工具, 抢到锁以后启动业务进程，失去 leader 之后 kill 业务进程。可以给 k8s 里面的服务添加一个通用的分布式锁功能，用户无需修改了业务代码。


## 示例
用法如下所示，给 pod 添加一个 init container 和一个 emptyDir，init container 中把 leader 通过 emptyDir 复制到业务容器内，然后在业务容器中通过 leader 抢锁并管理业务进程。

```
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    serving.knative.dev/release: "v0.8.0"
  name: controller
  namespace: knative-serving
spec:
  replicas: 2
  selector:
    matchLabels:
      app: controller
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "false"
      labels:
        app: controller
        serving.knative.dev/release: "v0.8.0"
    spec:
      initContainers:
      - name: leader
        image: "{{ .Values.controllerLeader.image.repository }}:{{ .Values.controllerLeader.image.tag }}"
        command:
        - sh
        args:
        - -c
        - |
          cp /bin/k8s-controller-leader /leader/k8s-controller-leader

        volumeMounts:
        - mountPath: /leader
          name: leader-dir
      containers:
      - name: controller
        command:
        - sh
        args:
        - -c
        - |
          mkdir -p /app/bin/
          start_script="/app/bin/app-start.sh"
          cat >${start_script} <<EOF
          #!/bin/sh
          /ko-app/controller
          EOF
          chmod +x ${start_script}

          /leader/k8s-controller-leader -app-command=${start_script} -leader-id=serving-controller

        env:
        - name: LEADER_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: SYSTEM_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: CONFIG_LOGGING_NAME
          value: config-logging
        - name: CONFIG_OBSERVABILITY_NAME
          value: config-observability
        - name: METRICS_DOMAIN
          value: knative.dev/serving
        image: "{{ .Values.controller.image.repository }}:{{ .Values.controller.image.tag }}"
        ports:
        - containerPort: 9090
          name: metrics
        resources:
          limits:
            cpu: 1000m
            memory: 1000Mi
          requests:
            cpu: 100m
            memory: 100Mi
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /leader
          name: leader-dir
      serviceAccountName: controller
      volumes:
      - name: leader-dir
        emptyDir: {}
```
