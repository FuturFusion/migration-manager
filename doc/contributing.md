% Include content from [../CONTRIBUTING.md](../CONTRIBUTING.md)
```{include} ../CONTRIBUTING.md
```

## Building locally
Building Migration Manager locally requires a current version of `mkosi` and `libnbd-dev`,
and should work on most Linux distributions. The UI additionally requires `yarn`.

After cloning the repository from GitHub, simply run:

    make install

This will build and install the daemon, CLI, and worker binaries, as well as the UI and worker image.

To only build the binaries, run:

    make

To only build the UI, run:

    make -C ui

To only build the worker image:

    make -C worker

Then, simply run `migration-managerd` to start the daemon, and `migration-manager` for the CLI.

## Testing
To run the test suite, run:

    make test
