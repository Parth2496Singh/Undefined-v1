# Go Learning & Mental Models

*This file will be updated with explanations of new Go concepts, such as `pgx` connection pooling, `http.ServeMux` patterns in Go 1.22+, and Go Workspace (`go.work`) structure as they are introduced.*

## In-Memory Job Queues vs. Naked Goroutines (Phase 2)
In Phase 2, we transitioned from spawning raw goroutines directly in the HTTP handler (`go runner.RunDeploy()`) to using an **In-Memory Job Queue** using buffered channels (`chan DeployJob`).

**The Mental Model:**
1. **The Risk of Naked Goroutines:** If an HTTP handler spins up a goroutine and passes it the `r.Context()`, the goroutine will instantly terminate if the client drops the HTTP connection before the task finishes. This causes "orphaned" Terraform processes.
2. **The Buffered Channel pattern:** 
   - A struct `JobQueue` owns a `chan DeployJob`.
   - On startup, `N` worker goroutines are spun up (e.g., `StartWorkers(5)`), actively ranging over `<-jobs`.
   - The HTTP handler quickly constructs a `DeployJob` struct, pushes it onto the channel (`q.jobs <- job`), and returns 200 OK.
   - The workers process jobs one-by-one. Since they were started from the `main` application scope (using `context.Background()`), their execution is completely decoupled from the HTTP request lifecycle.
   - *Limitation:* Without a persistent broker like NATS, if the server crashes, queued jobs in RAM are lost.
