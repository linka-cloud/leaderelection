mods := git gossip k8s s3

.PHONY: tidy
tidy:
	@go mod tidy
	@for mod in $(mods); do \
		cd $$mod && go mod tidy && cd ..; \
	done
