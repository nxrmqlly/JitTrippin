pipeline "test-lua" {
    description = "A test pipeline defined in Lua",
    visibility = "public",
    checkout = {
        url = "https://github.com/nxrmqlly/JitTrippin",
        branch = "master",
    },
    github = {
        push = {
            branch = "master",
        },
        release = {
            on = "published",
            artifacts = {
                {
                    job = "build",
                    name = "binary",
                    as = "jt.tar",
                },
            },
        },
    },

    job "debug" {
        image = "golang:latest",
        run "pwd && ls -la && find . -maxdepth 2 -type f | head -50",
    },

    job "build" {
        image = "golang:latest",
        env = {
            CGO_ENABLED = "0",
        },
        run "go build -o dist/jt ./cmd/jt",
        artifact("binary", "dist/jt"),
    },

    job "test" {
        image = "golang:latest",
        needs "build",
        run "go test ./...",
    },
}
