REGISTRY       ?= quay.io
ORG            ?= zvonkok
TAG            ?= $(git branch | grep \* | cut -d ' ' -f2)
IMAGE          ?= ${REGISTRY}/${ORG}/special-resource-operator:${TAG}
NAMESPACE      ?= openshift-sro
PULLPOLICY     ?= IfNotPresent
TEMPLATE_CMD    = sed 's+REPLACE_IMAGE+${IMAGE}+g; s+REPLACE_NAMESPACE+${NAMESPACE}+g; s+Always+${PULLPOLICY}+'
DEPLOY_SCC_RO   = manifests/0310_readonlyfs_scc.yaml
DEPLOY_OBJECTS  = deploy/namespace.yaml service_account.yaml role.yaml role_binding.yaml operator.yaml
DEPLOY_CRD      = deploy/crds/sro_v1alpha1_specialresource_crd.yaml
DEPLOY_CR       = deploy/crds/sro_v1alpha1_specialresource_cr.yaml

PACKAGE=github.com/zvonkok/special-resource-operator
MAIN_PACKAGE=$(PACKAGE)/cmd/manager

BIN=$(lastword $(subst /, ,$(PACKAGE)))

GOFMT_CHECK=$(shell find . -not \( \( -wholename './.*' -o -wholename '*/vendor/*' \) -prune \) -name '*.go' | sort -u | xargs gofmt -s -l)

DOCKERFILE=Dockerfile
IMAGE_TAG=zvonkok/special-resource-operator
IMAGE_REGISTRY=quay.io

ENVVAR=GOOS=linux CGO_ENABLED=0
GOOS=linux
GO_BUILD_RECIPE=GOOS=$(GOOS) go build -o $(BIN) $(MAIN_PACKAGE)

TEST_RESOURCES=$(shell mktemp -d)/test-init.yaml

all: build


build:
	$(GO_BUILD_RECIPE)

test-e2e: 
	@${TEMPLATE_CMD} manifests/service_account.yaml > $(TEST_RESOURCES)
	echo -e "\n---\n" >> $(TEST_RESOURCES)
	@${TEMPLATE_CMD} manifests/role.yaml >> $(TEST_RESOURCES)
	echo -e "\n---\n" >> $(TEST_RESOURCES)
	@${TEMPLATE_CMD} manifests/role_binding.yaml >> $(TEST_RESOURCES)
	echo -e "\n---\n" >> $(TEST_RESOURCES)
	@${TEMPLATE_CMD} manifests/operator.yaml >> $(TEST_RESOURCES)

	go test -v ./test/e2e/... -root $(PWD) -kubeconfig=$(KUBECONFIG) -tags e2e  -globalMan $(DEPLOY_CRD) -namespacedMan $(TEST_RESOURCES)

$(DEPLOY_CRD):
	@${TEMPLATE_CMD} $@ | kubectl apply -f -

deploy-crds: $(DEPLOY_CRD) 
	sleep 1

deploy-objects: deploy-crds
	for obj in $(DEPLOY_OBJECTS); do \
		$(TEMPLATE_CMD) deploy/$$obj | kubectl apply -f - ;\
	done	

deploy: deploy-objects
	@${TEMPLATE_CMD} $(DEPLOY_CR) | kubectl apply -f -

undeploy:
	for obj in $(DEPLOY_CRD) $(DEPLOY_CR) $(DEPLOY_OBJECTS)  do \
		$(TEMPLATE_CMD) $$obj | kubectl delete -f - ;\
	done	



verify:	verify-gofmt

verify-gofmt:
ifeq (, $(GOFMT_CHECK))
	@echo "verify-gofmt: OK"
else
	@echo "verify-gofmt: ERROR: gofmt failed on the following files:"
	@echo "$(GOFMT_CHECK)"
	@echo ""
	@echo "For details, run: gofmt -d -s $(GOFMT_CHECK)"
	@echo ""
	@exit 1
endif

clean:
	go clean
	rm -f $(BIN)

local-image:
ifdef USE_BUILDAH
	buildah bud $(BUILDAH_OPTS) -t $(IMAGE_TAG) -f $(DOCKERFILE) .
else
	sudo docker build -t $(IMAGE_TAG) -f $(DOCKERFILE) .
endif

test:
	go test ./cmd/... ./pkg/... -coverprofile cover.out

local-image-push:
ifdef USE_BUILDAH
	buildah push $(BUILDAH_OPTS) $(IMAGE_TAG) $(IMAGE_REGISTRY)/$(IMAGE_TAG)
else
	sudo docker tag $(IMAGE_TAG) $(IMAGE_REGISTRY)/$(IMAGE_TAG)
	sudo docker push $(IMAGE_REGISTRY)/$(IMAGE_TAG)
endif

.PHONY: all build generate verify verify-gofmt clean local-image local-image-push $(DEPLOY_CRDS) grafana

