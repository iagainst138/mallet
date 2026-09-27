SHELL=/bin/bash

GO_UNIT_TEST_FLAGS=-v
GO_TEST_CMD=go test ${GO_UNIT_TEST_FLAGS} -coverprofile ${UNIT_TEST_COVERAGE_DIR}/coverage.out ./...

VENV_NAME=.venv
VENV_PATH=testing/${VENV_NAME}
VENV_ACTIVATE=source ${VENV_PATH}/bin/activate

BIN_PATH=/tmp/mallet.out
SERVER_BIN_PATH=/tmp/test-server.out
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
UNIT_TEST_COVERAGE_DIR=${MAKEFILE_DIR}/.unit_test_coverage
FUNC_TEST_COVERAGE_DIR=${MAKEFILE_DIR}/.func_test_coverage
MERGED_COVERAGE_DIR=${MAKEFILE_DIR}/.merged_coverage


pipeline: unit_tests func_tests build


build:
	cd mallet && go build -o ${BIN_PATH} -ldflags="-s -w" cmd/main.go
	@echo ""


unit_tests:
	mkdir -p ${UNIT_TEST_COVERAGE_DIR}
	cd mallet && ${GO_TEST_CMD}
	@cd mallet && go tool cover -html ${UNIT_TEST_COVERAGE_DIR}/coverage.out -o ${UNIT_TEST_COVERAGE_DIR}/unit-test-coverage.html
	@echo -e "\n*** UNIT TESTS PASSED ***\n"


unit_tests_short:
	make unit_tests GO_UNIT_TEST_FLAGS="-v -short"


run-local-http: build
	ENABLE_PROFILING=1 ${BIN_PATH} \
		http \
		-a https://127.0.0.1:8080 \
		-g ./scripts/generator.py \
		-k \
		-n 20 \
		-e 1000 \
		-chart /tmp/chart.html \
		-logfile /tmp/mallet.log \
		-d 4s \
		-r 1s \
		-w 1s


build_test_server:
	cd test_server && go build -o ${SERVER_BIN_PATH} cmd/server/main.go


func_tests: certs ${VENV_PATH} build_test_server
	mkdir -p ${FUNC_TEST_COVERAGE_DIR}
	cd mallet && go build -cover -o ${BIN_PATH} -ldflags="-s -w" cmd/main.go
	${VENV_ACTIVATE} && cd testing/tests && GOCOVERDIR=${FUNC_TEST_COVERAGE_DIR} pytest --maxfail 1 .
	go tool covdata percent -i=${FUNC_TEST_COVERAGE_DIR} | column -t
	go tool covdata textfmt -i=${FUNC_TEST_COVERAGE_DIR} -o ${FUNC_TEST_COVERAGE_DIR}/profile.txt
	cd mallet && go tool cover -html ${FUNC_TEST_COVERAGE_DIR}/profile.txt -o ${FUNC_TEST_COVERAGE_DIR}/func-test-coverage.html
	@echo -e "\n*** FUNC TESTS PASSED ***\n"


merged_coverage:
	mkdir -p ${MERGED_COVERAGE_DIR}
	go tool covdata merge -i=${UNIT_TEST_COVERAGE_DIR},${FUNC_TEST_COVERAGE_DIR} -o ${MERGED_COVERAGE_DIR}
	go tool covdata textfmt -i=${MERGED_COVERAGE_DIR} -o ${MERGED_COVERAGE_DIR}/profile.txt
	cd mallet && go tool cover -html ${MERGED_COVERAGE_DIR}/profile.txt -o ${MERGED_COVERAGE_DIR}/merged-test-coverage.html


${VENV_PATH}:
	cd testing && python3 -m venv ${VENV_NAME}
	make venv


venv: ${VENV_PATH}
	${VENV_ACTIVATE} && pip install -r testing/requirements.txt


run-test-server: certs build_test_server
	${SERVER_BIN_PATH}


run-test-server-with-errors: certs build_test_server
	${SERVER_BIN_PATH} -with-errors -delay 1000ms


image: build
	docker build -t perf:0 .


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
		${UNIT_TEST_COVERAGE_DIR} \
		${FUNC_TEST_COVERAGE_DIR} \
		${MERGED_COVERAGE_DIR} \
		${SERVER_BIN_PATH}


profile_mem:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/heap?seconds=10


profile_cpu:
	go tool pprof -http :8001 -no_browser http://127.0.0.1:6060/debug/pprof/profile?seconds=10


ruff: ${VENV_PATH}
	${VENV_ACTIVATE} && ruff format
	${VENV_ACTIVATE} && ruff check --fix
