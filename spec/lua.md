# Lua for pipeline definitions

Pipelines are canonically defined in (.lua) files

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

This is valid too:

```lua
pipeline "name"
```

And so is this:

```lua


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
    image = "..."
    -- env, steps, dependencies
}
```

A job can contain:

- an image
- environment variables
- steps
- dependencies
- artifacts

Jobs are independent by default and can run in parallel

### Commands (`run`)

Two forms, but same meaning, the latter being nicer for extra metadata.

**An unnamed step:**

```lua
run = "go build ./..."
```

**A named step**

```lua
run "build-my-project" {
    cmd = "go build ./..."
}
```

### Dependencies (`needs`)

A Job may depend on other jobs:

```lua
job "package" {
    needs "compile", "test"

    run "make package"
}
```

Or, if the job also requires another job's artifacts:

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
env = {
    SOME_KEY = "COOL_VALUE"
}
```

### Artifacts (`artifact()`)

Artifacts are declared with:
The first argument is the artifact name and the second is its path.

```lua
artifact("binary", "dist/app")
```

## GitHub (`github`)

GitHub-specific behaviour is defined here.
(Requires the repository and the branch to be tracked by a `jtd` instance)

```lua
pipeline "build" {...}
github {
    push = {
        branch = "main",
    },
}
    -- jobs...
}
```

### Push Triggers (`push`)

Pushes may be filtered by branch or tag:

```lua
github {
    push = {
        branch = "main",
    },
}
```

```lua
github {
    push = {
        tag = "v*", -- globbing!
    },
}
```

Or, using multiple filters:

```lua
github {
    push = {
        branch = { "main", "develop" },
        tag = "v*",
    },
    -- ...release
}
```

### Release Triggers (`release`)

A pipeline may respond to published GitHub releases:

```lua
github {
    push = {...}
    release = {
        on = "published",
        artifacts = {
            {
                job = "build",
                name = "binary",
                as = "app-linux.tar",
            },
        },
    },
}
```

`on` specifies the GitHub release action, should be `"published"` in most cases.

## The magic of using Lua
