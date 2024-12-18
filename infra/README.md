# Get Started

**Install argo**
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

**Portforward (or use k9s)**
```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

**Get admin secret**
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath=”{.data.password}” | base64 -d
```

After this point, argo should pull latest updates from GH directly.
