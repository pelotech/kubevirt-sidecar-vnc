#!/bin/bash

printf "Process A\n"
./main &

printf "Process B\n"
./sidecar-shim --version v1alpha3 &

# Wait for any process to exit
wait -n

# Exit with status of process that exited first
exit $?