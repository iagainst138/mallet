SHELL=/bin/bash

GO_UNIT_TEST_FLAGS=
GO_TEST_CMD=go test ${GO_UNIT_TEST_FLAGS} .

VENV_NAME=.venv
VENV_PATH=testing/${VENV_NAME}
VENV_ACTIVATE=source ${VENV_PATH}/bin/activate


build:
	cd load_gen && go build -o load-gen -ldflags="-s -w" cmd/main.go
	@echo ""


test_pacer:
	cd pacer && ${GO_TEST_CMD}


unit_tests: test_pacer


unit_tests_short:
	make unit_tests GO_UNIT_TEST_FLAGS="-v -short"


test:
	echo "*** placeholder for ci job ***"


#build_linux:
#	CC="$$(./scripts/determine-c-compiler.sh)" \
#		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 make build


build_test_server:
	cd test_server && go build -o ../test-server cmd/server/main.go


build_har_tool:
	go build -o har-tool har/main.go


build_latency_tool:
	go build -o latency-tool ./latency/cmd/latency.go


func_tests: certs ${VENV_PATH} build build_test_server build_latency_tool
	${VENV_ACTIVATE} && cd testing/tests && pytest --maxfail 1 .


${VENV_PATH}:
	cd testing && python3 -m venv ${VENV_NAME}


venv: ${VENV_PATH}
	${VENV_ACTIVATE} && pip install -r testing/requirements.txt


run_test_server: certs build_test_server
	./test-server


run_test_server_with_errors: certs build_test_server
	./test-server -with-errors -delay 100ms


image: build
	docker build -t perf:0 .


pipeline: unit_tests func_tests


certs:
	mkdir -p certs
	openssl \
		req \
		-new \
		-x509 \
		-newkey rsa:4096 \
		-keyout certs/key.pem \
		-out certs/cert.pem \
		-sha256 \
		-days 3650 \
		-nodes \
		-addext "subjectAltName=DNS:localhost,DNS:127.0.0.1,IP:127.0.0.1" \
		-subj "/C=XX/ST=StateName/L=CityName/O=CompanyName/OU=CompanySectionName/CN=CommonNameOrHostname"


clean:
	rm -rf \
		certs \
		${VENV_PATH} \
		load_gen/load-gen


profile_mem:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/heap?seconds=20


profile_cpu:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/profile?seconds=20


ruff:
	ruff format
	ruff check --fix
