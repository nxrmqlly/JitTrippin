pipeline "release" {
    description = "Cross-compile and attach binaries to GitHub releases",
    visibility = "public",
}

checkout {
    url = "https://github.com/nxrmqlly/JitTrippin",
    branch = "master",
}

local targets = {
    { os = "linux",   arch = "amd64" },
    { os = "linux",   arch = "arm64" },
    { os = "darwin",  arch = "amd64" },
    { os = "darwin",  arch = "arm64" },
    { os = "windows", arch = "amd64" },
}

local release_artifacts = {}

for _, t in ipairs(targets) do
    local suffix = t.os .. "-" .. t.arch
    local ext = t.os == "windows" and ".exe" or ""
    local name = "jt-" .. suffix .. ext

    job("build-" .. suffix) {
        image = "golang:latest",
        env = {
            GOOS = t.os,
            GOARCH = t.arch,
            CGO_ENABLED = "0",
        },

        run "compile" { cmd = "go build -o " .. name .. " ./cmd/jt" },

        artifact(name, name),
    }

    table.insert(release_artifacts, {
        job = "build-" .. suffix,
        name = name,
        as = name,
    })
end

github {
    push = {
        tag = "v*",
    },

    release = {
        on = "published",
        artifacts = release_artifacts,
    },
}
