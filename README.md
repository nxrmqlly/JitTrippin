<center><h1>JitTrippin (JT)</h1></center>
<img src="./assets/jt-banner-thin.png" alt="JitTrippin Banner">

CI/CD Engine for the average monkeybrain. minus YAML.

JitTrippin is designed to but plug and play with whatever environment you please.
It's designed to not lock you down into using only docker, or S3.

- You can choose the enviroment you run your pipelines. (ie. Docker, Podman, MicroVM etc.)
- You can choose whatever Storage method you want to use for your artifacts. (ie. Disk, S3 etc.)

Ofcourse, choices only exist if they are implemented.
JitTrippin is in very early development. We have Docker + Disk Storage (for artifacts) for now.

## Compatibility

JitTrippin is designed for unix-like systems (Linux, MacOS) primarily. Windows support is untested.
No support is provided for broken JT on Windows machines.

JT also assumes that Pipelines will run in Linux containers. Support for Windows containers is
best-effort.
