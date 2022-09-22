# Kubernetes 镜像预热


通过在指定节点运行Pod进行镜像预热，支持边缘节点


## 快速开始

1. 安装 image-preload
   ```shell
   kubectl apply -f image-preload.yaml
   ```
2. 创建 ImagePullJob 并安装
    ```yaml
    apiVersion: job.wuxs.vip/v1
    kind: ImagePullJob
    metadata:
      name: test-job
    spec:
      selector:
        matchLabels:
          location: edge
      images:
        - nginx:latest
        - redis:latest
        - mysql:8.0
    ```
3. 等待 Pod 运行结束