SHELL=/bin/bash

GO_UNIT_TEST_FLAGS=-v
GO_TEST_CMD=go test ${GO_UNIT_TEST_FLAGS} ./...

VENV_NAME=.venv
VENV_PATH=testing/${VENV_NAME}
VENV_ACTIVATE=source ${VENV_PATH}/bin/activate

BIN_PATH=/tmp/mallet.out
SERVER_BIN_PATH=/tmp/test-server.out


build:
	cd mallet && go build -o ${BIN_PATH} -ldflags="-s -w" cmd/main.go
	@echo ""


unit_tests:
	cd mallet && ${GO_TEST_CMD}
	@echo -e "\n*** UNIT TESTS PASSED ***\n"


unit_tests_short:
	make unit_tests GO_UNIT_TEST_FLAGS="-v -short"


run_local_http: build
	ENABLE_PROFILING=1 ${BIN_PATH} \
		http \
		-a https://127.0.0.1:8080 \
		-g ./scripts/generator.py \
		-k \
		-n 200 \
		-e 1000 \
		-chart /tmp/chart.html \
		-d 8s \
		-r 1s \
		-w 1s


build_test_server:
	cd test_server && go build -o ${SERVER_BIN_PATH} cmd/server/main.go


func_tests: certs ${VENV_PATH} build build_test_server
	${VENV_ACTIVATE} && cd testing/tests && pytest --maxfail 1 .
	@echo -e "\n*** FUNC TESTS PASSED ***\n"


${VENV_PATH}:
	cd testing && python3 -m venv ${VENV_NAME}
	make venv


venv: ${VENV_PATH}
	${VENV_ACTIVATE} && pip install -r testing/requirements.txt


run_test_server: certs build_test_server
	${SERVER_BIN_PATH}


run_test_server_with_errors: certs build_test_server
	${SERVER_BIN_PATH} -with-errors -delay 100ms


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
		${BIN_PATH} \
		${SERVER_BIN_PATH}


profile_mem:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/heap?seconds=20


profile_cpu:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/profile?seconds=20


ruff: ${VENV_PATH}
	${VENV_ACTIVATE} && ruff format
	${VENV_ACTIVATE} && ruff check --fix
