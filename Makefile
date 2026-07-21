# Quick local loop for the kind cluster. Not fancy, just what I actually run.

CLUSTER := gitops-reconciler

kind-up:
	kind create cluster --name $(CLUSTER)
	kubectl create namespace staging || true
	kubectl create namespace prod || true

kind-down:
	kind delete cluster --name $(CLUSTER)

build-images:
	docker build -t gitops-reconciler-web:local services/web
	docker build -t gitops-reconciler-api:local services/api

load-images: build-images
	kind load docker-image gitops-reconciler-web:local --name $(CLUSTER)
	kind load docker-image gitops-reconciler-api:local --name $(CLUSTER)

apply-raw: load-images
	kubectl apply -f k8s-raw/namespaces.yaml
	kubectl apply -f k8s-raw/staging
	kubectl apply -f k8s-raw/prod

helm-staging:
	helm upgrade --install gitops-reconciler-staging charts/app -f envs/staging/values.yaml -n staging --create-namespace

helm-prod:
	helm upgrade --install gitops-reconciler-prod charts/app -f envs/prod/values.yaml -n prod --create-namespace

install-argocd:
	kubectl create namespace argocd || true
	kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

argocd-ui:
	kubectl -n argocd port-forward svc/argocd-server 8081:443

argocd-password:
	kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

apply-root-app:
	kubectl apply -f envs/root-app.yaml

.PHONY: kind-up kind-down build-images load-images apply-raw helm-staging helm-prod install-argocd argocd-ui argocd-password apply-root-app
