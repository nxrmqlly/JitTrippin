# Lua for pipeline definitions

Pipelines are canonically defined in (.lua) files

- `pipeline`: pipeline metadata
- `checkout`: source repository
- `job`: units of work
    - `image`: execution environment image
    - `run`: commands executed by a job
    - `needs`: job dependencies
    - `env`: environment variables
    - `artifact`: files produced by a job
- `github`: GitHub-specific triggers and release behavior

**Since Lua is a programming language, you get access to local variables, loops, if-else statements and more.**

## Pipeline (`pipeline`)

A pipeline has a name and some optional metadata, it sits at the top level of a lua file.
There can be only one pipeline per file.

```lua
pipeline "name" {                 -- must be a name
    description = "do something", -- optional
    visibility = "public"         -- optional (default: private)
}
-- jobs, checkout, artifacts
```

Both `description` and `visibility` are optional.

This is valid too:

```lua
pipeline "name"
```

And so is this:

```lua
pipeline "name" {}
```

## Checkout (`checkout`)

`checkout` defines a git repository that should be cloned + checked out before running the pipeline.

```lua
checkout {
    url = "https://github.com/example/project",
    branch = "main",
}
```

- `url`: repository URL
- `branch`: branch or ref to check out

## Jobs (`job`)

Jobs are the basic units of work in a pipeline

```lua
pipeline "name" {...}

job "job-name" {
    image = "golang:latest"

    run "go build ./..."
    run "echo zhifubao!"
    -- and more steps
}

job "some-other-job" {
    image = "alpine:latest"
    -- env, steps, dependencies
}
```

A job can contain:

- an image
- environment variables
- steps
- dependencies
- artifacts

**Jobs are independent by default and can run in parallel**

### Execution image (`image`)

The image field specifies the container image used to execute the job.

```lua
job "build" {
    image = "golang:latest",
    ...
}
```

### Commands (`run`)

Two forms, but same meaning, the latter being nicer for extra metadata.

**An unnamed step:**

```lua
run "go build ./..."
```

**A named step**

```lua
run "build-my-project" {
    cmd = "go build ./..."
}
```

Multiple steps run in order of declaration:

```lua
job "build" {
    image = "golang:latest",

    run "compile" {
        cmd = "go build ./...",
    },

    run "test" {
        cmd = "go test ./...",
    },
}
```

i.e. `compile` step runs before `test` step

### Dependencies (`needs`)

A Job may depend on other jobs:

```lua
job "package" {
    needs "compile", "test"

    run "make package"
}
```

Optionally, if the job also requires another job's artifacts:

```lua
job "package" {
    needs "compile" {
        requires = { "binary" }
    }
}
```

### Environment (`env`)

Jobs can also have environment variables associated with them.

```lua
job "build" {
    image = "golang:latest",
    env = {
        CGO_ENABLED = "0",
        GOOS = "linux",
        GOARCH = "amd64",
    },

    ...
}
```

### Artifacts (`artifact()`)

Artifacts are declared and made available to JT after the job completes:

```lua
job "build" {
    ...
    artifact("binary", "dist/app")
}
```

`artifact` takes two positional arguments

1. artifact name
2. path to produced file

```lua
artifact("binary", "dist/app")
```

The artifact name can later be referenced by other jobs or integrations by name.

## GitHub (`github`)

GitHub-specific behaviour is defined here.
(Requires the repository and the branch to be tracked by a `jtd` instance)

```lua
github {
    push = {
        branch = "main",
    },
}
```

### Push Triggers (`push`)

Pushes may be filtered by branch:

```lua
github {
    push = {
        branch = "main",
    },
}
```

Or tag:

```lua
github {
    push = {
        tag = "v*", -- globbing!
    },
}
```

Or, using multiple filters, in arrays:

```lua
github {
    push = {
        branch = { "main", "develop" },
        tag = { "v*", "rc-*" } -- also globbing here!
    },
}
```

Patterns support glob matching.

### Release Triggers (`release`)

A pipeline may respond to published GitHub releases:

```lua
github {
    release = {
        on = "published",
        artifacts = {
            { job = "build", name = "binary", as = "app-linux.tar" },
        },
    },
}
```

> `on` specifies the GitHub release action, should be `"published"` in most cases.

### Release Artifacts

Release artifacts specify which job artifacts should be attached to the published GitHub release.

```lua
artifacts = {
    {
        job = "build",
        name = "binary",
        as = "app-linux.tar",
    },
}
```

- `job`: name of job that produced the artifact
- `name`: artifact name (w.r.t. that job)
- `as`: filename to use for the release asset

`as` is optional, defaults to the original artifact name

## Lua Features

### Local Variables

```lua
local image = "golang:latest"

job "build" {
    image = image,
    run "go build ./...",
}
```

### Loops

**Paticularly useful for matrix style builds**

```lua
local targets = {
    { os = "linux", arch = "amd64" },
    { os = "linux", arch = "arm64" },
    { os = "darwin", arch = "arm64" },
}

for _, target in ipairs(targets) do
    local name = target.os .. "-" .. target.arch

    job("build-" .. name) {
        image = "golang:latest",

        env = {
            GOOS = target.os,
            GOARCH = target.arch,
            CGO_ENABLED = "0",
        },

        run "go build ./...",
    }
end
```

This produces 3 jobs:

```
build-linux-amd64
build-linux-arm64
build-darwin-arm64
```

### Conditional configuration

Lua can be used to conditionally construct configuration.

```lua
local debug = false

if debug then
    job "debug" {
        image = "golang:latest",
        run "go test -v ./...",
    }
end
```

Lua tables can be used to construct configuration before passing it to JT's DSL functions.

```lua
local targets = {
    { os = "linux", arch = "amd64" },
    { os = "linux", arch = "arm64" },
    { os = "darwin", arch = "arm64" },
}

local release_artifacts = {}

for _, target in ipairs(targets) do
    local suffix = target.os .. "-" .. target.arch

    job("build-" .. suffix) {
        image = "golang:latest",

        env = {
            GOOS = target.os,
            GOARCH = target.arch,
            CGO_ENABLED = "0",
        },

        run "go build ./...",
        artifact("binary", "dist/app"),
    }

    table.insert(release_artifacts, {
        job = "build-" .. suffix,
        name = "binary",
        as = "app-" .. suffix,
    })
end

github {
    release = {
        on = "published",
        artifacts = release_artifacts,
    },
}
```

## Complete Examples

The CI for JitTrippin is done via... JitTrippin.
Some of its own CI pipelines can be found in the [.jt](./.jt) directory.  