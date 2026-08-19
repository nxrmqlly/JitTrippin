pipeline "hello" {}

job "hello" {
    image = "alpine:latest",

    run "echo hello world",
    run "echo -----------"
}
