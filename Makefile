all: leader

leader:
	@echo "build leader image"
	./build/build-image.sh leader
