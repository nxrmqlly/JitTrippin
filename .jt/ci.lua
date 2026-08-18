pipeline "ci" {
    description = "Lint, vet, and build on every push",
    visibility = "public",
}

checkout {
    url = "https://github.com/nxrmqlly/JitTrippin",
    branch = "master",
}

github {
    push = {
        branch = "master",
    },
}

job "lint" {
    image = "golang:latest",
    run "go vet ./..."
}

job "build" {
    image = "golang:latest",
    env = {
        CGO_ENABLED = "0"
    },

    run "build-cli" {
        cmd = "CGO_ENABLED=0 go build -o dist/jt ./cmd/jt"
    },

    run "build-daemon" {
        cmd = "CGO_ENABLED=0 go build -o dist/jtd ./cmd/jtd"
    },
    artifact("jt-cli", "dist/jt"),
    artifact("jtd-daemon", "dist/jtd")
}
