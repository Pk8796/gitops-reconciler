package main

deny[msg] {
	input.kind == "Deployment"
	container := input.spec.template.spec.containers[_]
	endswith(container.image, ":latest")
	msg := sprintf("container '%s' uses mutable ':latest' tag (image=%s) - use the commit SHA", [container.name, container.image])
}
