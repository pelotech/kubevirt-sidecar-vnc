# KubeVirt Sidecar VNC

```shell
docker buildx build --platform linux/amd64 -t kubevirt-sidecar-vnc .
docker tag kubevirt-sidecar-vnc ghcr.io/chomatdam/kubevirt-sidecar-vnc:latest
docker push ghcr.io/chomatdam/kubevirt-sidecar-vnc:latest 
```