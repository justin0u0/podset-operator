# podset-operator

**使用 operator-sdk 建立一個 CustomResourceDefinition，類似於 ReplicaSet 的功能，在這裡筆者將他稱為 PodSet。**

PodSet 可以自動的創建 Replicas 的數量個 Pods，並且在 Pods 被新增、刪除，或是 PodSet 的 Spec 被修改時自動增減 Pods 的數量。

先附上 PodSet 的 Spec：

```yaml
apiVersion: k8stest.justin0u0.com/v1alpha1
kind: PodSet
metadata:
  name: podset-sample
spec:
  # Add fields here
  replicas: 2
```

部落格文章：https://blog.justin0u0.com/%E4%BD%BF%E7%94%A8-operator-sdk-%E5%9C%A8-Kubernetes-%E4%B8%AD%E5%AF%A6%E4%BD%9C-CRD-%E4%BB%A5%E5%8F%8A-Controller/
