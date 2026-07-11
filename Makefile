.PHONY: cluster destroy infra deploy hooks

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
