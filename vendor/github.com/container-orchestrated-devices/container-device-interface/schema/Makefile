VALIDATE ?= ../bin/validate
SCHEMA ?= schema.json

test:
	@FMT_RED=$$(tput setaf 1); \
	FMT_BLUE=$$(tput setaf 12); \
	FMT_CLEAR=$$(tput sgr0); \
	echo "Running Good Tests"; \
	for FILE in $$(ls "testdata/good"); do \
		FILE_PATH="testdata/good/$${FILE}"; \
		if $(VALIDATE) --schema "$(SCHEMA)" "$${FILE_PATH}" > /dev/null ; then \
			printf '%s[OK]%s %s\n' "$${FMT_BLUE}" "$${FMT_CLEAR}" "$${FILE_PATH}"; \
		else \
			printf '%s[KO]%s %s\n' "$${FMT_RED}" "$${FMT_CLEAR}" "$${FILE_PATH}"; \
			exit 1; \
		fi \
	done; \
	echo "Running Bad Tests"; \
	for FILE in $$(ls "testdata/bad"); do \
		FILE_PATH="testdata/bad/$${FILE}"; \
		if $(VALIDATE) --schema "$(SCHEMA)" "$${FILE_PATH}" > /dev/null ; then \
			printf '%s[KO]%s %s\n' "$${FMT_RED}" "$${FMT_CLEAR}" "$${FILE_PATH}"; \
			exit 1; \
		else \
			printf '%s[OK]%s %s\n' "$${FMT_BLUE}" "$${FMT_CLEAR}" "$${FILE_PATH}"; \
		fi \
	done
