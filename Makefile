.PHONY: cluster destroy infra deploy deploy-chat deploy-users argocd-bootstrap hooks

hooks:
	git config core.hooksPath .githooks

cluster:
	k3d cluster create --config infra/k3d/config.yaml

destroy:
	k3d cluster delete vidcast

infra:
	cd infra/terraform/envs/local && terraform init && terraform apply -auto-approve

deploy:
	docker build -t localhost:5001/echo:dev services/echo
	docker push localhost:5001/echo:dev
	helm dependency update deploy/charts/echo
	helm upgrade --install echo deploy/charts/echo -n apps --create-namespace
	kubectl rollout status deployment/echo -n apps --timeout=60s

deploy-chat:
	docker build -t localhost:5001/chat:dev services/chat
	docker push localhost:5001/chat:dev
	helm dependency update deploy/charts/chat
	helm upgrade --install chat deploy/charts/chat -n apps --create-namespace
	kubectl rollout status deployment/chat -n apps --timeout=60s

deploy-users:
	docker build -t localhost:5001/users:dev services/users
	docker push localhost:5001/users:dev
	helm dependency update deploy/charts/users
	helm upgrade --install users deploy/charts/users -n apps --create-namespace
	kubectl rollout status deployment/users -n apps --timeout=60s

# One-time: after `make infra` installs ArgoCD, point it at this repo's
# deploy/argocd/apps directory. ArgoCD reconciles everything under there
# from then on - no further manual `helm upgrade --install` needed for
# services onboarded to GitOps.
argocd-bootstrap:
	kubectl apply -f deploy/argocd/root-app.yaml
