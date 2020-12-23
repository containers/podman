# -*- bash -*-

test_port 5000 = "Podman rulez!"
podman container inspect -l --format '{{.Config.Labels}}' | grep "the_best"
