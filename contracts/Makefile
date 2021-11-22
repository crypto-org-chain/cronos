SOLC_VERSION := $(shell which solc)
all    :; dapp --use ${SOLC_VERSION} build
clean  :; dapp clean
test   :; dapp --use ${SOLC_VERSION} test
deploy :; dapp create CronosERC20
