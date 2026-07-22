#!/bin/bash

# dereferences the source JSON schemas resolving all definitions and producing flat
# json schema files that's easy to load remotely and validate as they are standalone
# single files.
#
# this is also run as part of 'go generate'

set -e

cd "$(dirname "$0")"

go run api/gen_dereference.go