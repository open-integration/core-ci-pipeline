echo "Creating secret in kuberentes cluster"

kubectl apply -f assets/kubernetes/pvc.yaml || true
kubectl apply -f assets/kubernetes/job.yaml

