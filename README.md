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

## Roadmap

(in best-effort order of priority, or what's more fun to implement)

- [x] JSON Parser
- [x] Executor (w/ worker pools)
- [x] Validator
- [x] Scheduler
- [x] Live Stderr/Stdout
- [x] Runner Core
- [x] Artifacts Core
- [X] Checkout Core
- [ ] CLI (🔄️ In progress)
- [ ] CLI: Pretty printing
- [ ] API (🔄️ In progress)
- [x] API: Auth!
- [x] API: Logs Streaming
- [ ] API: HTTP Error Handling
- [ ] API: Authorization 
- [ ] Engine: Cache
- [ ] Engine: Secrets
- [ ] Engine: Annotated Logs
- [ ] Engine: Retry / Timeout
- [ ] Engine: S3 Artifact Store
- [ ] Custom Logger
- [ ] Engine: Variables
- [ ] Engine: Matrix builds
- [ ] ~~Engine: Weighted DAG~~
