# gitops-reconciler

A tiny two-service app deployed with GitOps: Git is the source of truth, Argo CD
continuously reconciles the cluster to match it, and drift gets detected (and, in
staging, auto-corrected) instead of silently rotting.

> "A cluster that continuously reconciles itself to Git - change production by
> hand and it heals back to the declared state on its own."

## What's actually here (be honest about this in interviews)

- **Rung 0** - hand-written `kubectl apply` manifests in `k8s-raw/` for both envs.
- **Rung 1** - one Helm chart (`charts/app`) with per-environment `values.yaml`.
- **Rung 2** - Argo CD app-of-apps (`envs/root-app.yaml` -> `envs/staging` + `envs/prod`),
  automated sync, drift detection, self-heal **on in staging, off in prod**.
- **Rung 3** - GitHub Actions CI: compile check -> policy gate -> build & push images
  to GHCR tagged with the commit SHA -> auto-bump staging's `values.yaml`.
- **Rung 4** - OPA/Conftest policy gate in CI (`policies/`) blocking `:latest` tags,
  missing resource limits, and privileged containers.

**Not built yet:** Argo Rollouts canary + auto-rollback (Rung 6), Terraform-provisioned
infra (Rung 7), Prometheus/Grafana dashboards (Rung 5 - the `/metrics` endpoints are
there and ready to be scraped, just nothing scraping them yet). Only claim what's
actually running.

## Why GitOps (the interview answer)

Running `kubectl apply` from a laptop, or pushing deploys from a CI script, has
three problems this fixes:

1. **No single source of truth.** Nobody can say for certain what's actually
   running vs. what should be. GitOps makes Git authoritative and continuously
   reconciles the cluster to match it.
2. **Configuration drift.** Someone hotfixes prod by hand, live state quietly
   diverges from Git, nobody notices until it bites them. This gets detected,
   and in low-stakes environments, auto-healed.
3. **Risky all-at-once releases.** Not solved here yet (that's the canary rung),
   but it's the reason Argo Rollouts is the next thing to add, not a random pick.

## Architecture

```
Git repo
  services/        - web + api source, each with a multi-stage Dockerfile
  charts/app/       - one Helm chart, shared by every environment
  envs/staging/     - staging values.yaml + Argo CD Application
  envs/prod/        - prod values.yaml + Argo CD Application
  policies/         - Conftest/OPA rules run in CI
        |  git push / PR
        v
GitHub Actions: compile check -> conftest policy gate -> build+push image -> bump staging tag
        |  Argo CD watches the repo, no push from CI into the cluster
        v
kind cluster (or EKS/GKE later, unchanged manifests)
  Argo CD  -- reconciles + self-heals --> staging namespace
           -- reconciles, alerts only --> prod namespace
```

## Prerequisites

- Docker
- [kind](https://kind.sigs.k8s.io/)
- `kubectl`
- `helm` v3
- Go 1.22 (only needed if you want to run the services outside a container)

## Running it locally

### Option A - raw manifests (Rung 0, no Helm/Argo yet)

```bash
make kind-up
make apply-raw
kubectl -n staging get pods
kubectl -n staging port-forward svc/web 8080:8080
curl "localhost:8080/?url=https://sjsu.edu"
```

Kill a pod and watch Kubernetes bring it back on its own:

```bash
kubectl -n staging delete pod -l app=api
kubectl -n staging get pods -w
```

### Option B - Helm (Rung 1)

```bash
make kind-up
make load-images
make helm-staging
make helm-prod
helm template staging charts/app -f envs/staging/values.yaml   # sanity check output
```

### Option C - full GitOps loop (Rung 2)

```bash
make kind-up
make install-argocd
make argocd-password        # log into the UI with this
make argocd-ui               # https://localhost:8081

# push charts/app + envs/ to your own GitHub repo first, then:
kubectl apply -f envs/root-app.yaml
```

Once Argo CD has synced, try the drift demo:

```bash
# staging: self-heal is ON
kubectl -n staging edit deployment web   # bump replicas by hand, save
# watch Argo CD flag it OutOfSync, then revert it back to what Git says

# prod: self-heal is OFF on purpose
kubectl -n prod edit deployment web       # your change sticks
# Argo CD shows OutOfSync but does not touch it - a human decides what to do
```

## Design decisions worth defending

- **Self-heal on in staging, off in prod.** Auto-reverting is great until
  someone's mid-incident and needs a hand-applied hotfix to survive for the
  next ten minutes. Drift is *observable* everywhere; it's only
  *auto-corrected* where being wrong is cheap.
- **Image tags are never `:latest`.** Every image is tagged with its commit
  SHA. `policies/deny_latest_tag.rego` enforces this in CI so it can't
  regress by accident.
- **CI never touches the cluster.** GitHub Actions is push-based (lint, gate,
  build, publish). Argo CD is pull-based (it reaches into Git from inside the
  cluster). Deploy credentials never leave the cluster, and it's what makes
  drift detection possible in the first place - if CI could push, "what's
  actually running" would be ambiguous again.
- **Staging tag bumps are automatic, prod is a manual PR.** Promotion to prod
  is a deliberate, reviewable Git change (bump `envs/prod/values.yaml` to a
  SHA that's already proven itself in staging), not something CI does for you.

## Known gotchas

- Argo CD's `helm.valueFiles` here reference a path outside the chart
  directory (`../../envs/staging/values.yaml`). This works because Argo CD
  clones the whole repo and resolves the path relative to `spec.source.path`,
  but it trips people up the first time - worth mentioning if asked.
- If Argo CD loses connectivity to Git, it holds the last-known-good desired
  state and just stops reconciling toward new commits. It does not tear
  anything down. Syncing resumes automatically once Git is reachable again.
- `envs/*/values.yaml` ship with `tag: init` as a placeholder so a bare
  `helm template` doesn't immediately trip the `:latest` policy before CI has
  ever run. The first successful push to `main` overwrites it for real.

## Metrics worth capturing (for a demo recording)

Time from commit to sync; time from drift to self-heal; pod recovery time
after `kubectl delete pod`; CI pipeline pass rate.

## Resume bullets (only claim what you built)

- Built a GitOps-driven, multi-environment (staging/prod) Kubernetes platform
  with **Argo CD**, implementing automated **drift detection and self-healing**
  to keep live state reconciled to Git as the single source of truth.
- Packaged workloads with **Helm**, using environment-specific values for
  portable staging/prod deployments from a single chart.
- Enforced **policy-as-code** (**OPA/Conftest**) in a **GitHub Actions**
  pipeline, automatically blocking non-compliant manifests (unpinned tags,
  missing resource limits, privileged containers).

## Interview Q&A

**Push vs pull deploys - which is which here?**
GitHub Actions is push-based CI (lint/gate/build/publish). Argo CD pulls
desired state from Git and applies it from inside the cluster. Pull keeps
deploy credentials in the cluster and treats Git as continuously authoritative,
which is what makes drift detection possible at all.

**Would you enable self-heal in production?**
Not unconditionally - it can fight an emergency hotfix. Prod detects and
surfaces drift as OutOfSync but doesn't auto-revert; a human decides.
Auto-correct only where the blast radius is low.

**What happens if Argo CD can't reach Git?**
It holds the last known desired state and stops reconciling toward new
changes; it does not tear down what's running. Syncs resume when Git returns.

## Explicitly out of scope

No service mesh, no multi-cluster, no databases, no message queue. Depth over
breadth - adding these trades a defensible project for a fragile tool-pile.
