package main

import (
        "flag"
        "fmt"
        "io"
        "net/http"
        "os"
        "os/exec"
        "path/filepath"
        "strings"
)

var (
        nworkers    = flag.Int("n", 4, "The number of workers to start")
        workQueue   = make(chan workRequest, 100)
        workerQueue chan chan workRequest
)

type workRequest struct {
        Cmd string
}

type worker struct {
        ID          int
        work        chan workRequest
        workerQueue chan chan workRequest
        QuitChan    chan bool
}

func execCmd(cmd string) {
        fmt.Println("command is ", cmd)
        // splitting the pythonhead => g++ parts => rest of the command
        parts := strings.Fields(cmd)
        head := parts[0]
        parts = parts[1:len(parts)]

        os.Chdir(filepath.Dir(parts[1]))

        out, err := exec.Command(head, parts...).Output()
        if err != nil {
                fmt.Printf("%s", err)
        }
        fmt.Printf("%s", out)
}

// newworker creates, and returns a new worker object. Its only argument
// is a channel that the worker can add itself to whenever it is done its
// work.
func newworker(id int, workerQueue chan chan workRequest) worker {
        // Create, and return the worker.
        worker := worker{
                ID:          id,
                work:        make(chan workRequest),
                workerQueue: workerQueue,
                QuitChan:    make(chan bool)}

        return worker
}

// This function "starts" the worker by starting a goroutine, that is
// an infinite "for-select" loop.
func (w *worker) Start() {
        go func() {
                for {
                        // Add ourselves into the worker queue.
                        w.workerQueue <- w.work

                        select {
                        case work := <-w.work:
                                // Receive a work request.
                                fmt.Printf("worker%d: Received work request\n", w.ID)
                                execCmd(work.Cmd)
                        case <-w.QuitChan:
                                // We have been asked to stop.
                                fmt.Printf("worker%d stopping\n", w.ID)
                                return
                        }
                }
        }()
}

// Stop tells the worker to stop listening for work requests.
// Note that the worker will only stop *after* it has finished its work.
func (w *worker) Stop() {
        go func() {
                w.QuitChan <- true
        }()
}

func startDispatcher(nworkers int) {
        // First, initialize the channel we are going to but the workers' work channels into.
        workerQueue = make(chan chan workRequest, nworkers)

        // Now, create all of our workers.
        for i := 0; i < nworkers; i++ {
                fmt.Println("Starting worker", i+1)
                worker := newworker(i+1, workerQueue)
                worker.Start()
        }

        go func() {
                for {
                        select {
                        case work := <-workQueue:
                                fmt.Println("Received work requeust")
                                go func() {
                                        worker := <-workerQueue

                                        fmt.Println("Dispatching work request")
                                        worker <- work
                                }()
                        }
                }
        }()
}

func main() {
        fmt.Println("Starting the dispatcher")
        startDispatcher(*nworkers)

        mux := http.NewServeMux()
        mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != "POST" {
                        w.Header().Set("Allow", "POST")
                        w.WriteHeader(http.StatusMethodNotAllowed)
                        return
                }
                path := r.URL.Path
                if path == "/" {
                        io.WriteString(w, "hi")
                } else if path == "/task/add/" {
                        if cmd := r.FormValue("cmd"); cmd != "" {
                                work := workRequest{Cmd: cmd}
                                workQueue <- work
                            io.WriteString(w, "started task")
                        } else {
                            io.WriteString(w, "error starting task")
            }
                } else {
                        io.WriteString(w, "ok")
                }
        })
        http.ListenAndServe("127.0.0.1:8080", mux)
}
