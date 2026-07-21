package main

deny[msg] {
	input.kind == "Deployment"
	container := input.spec.template.spec.containers[_]
	container.securityContext.privileged == true
	msg := sprintf("container '%s' runs privileged - not allowed by policy", [container.name])
}
