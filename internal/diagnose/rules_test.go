package diagnose

import (
	"strings"
	"testing"
)

func TestDiagnoseDefaultNormalizesAndMatchesKnownErrors(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want string
	}{
		{name: "CUDA OOM", job: Job{Log: "\x1b[31mCUDA Error: Out   Of Memory\x1b[0m"}, want: "CUDA/GPU memory exhausted"},
		{name: "CUDA runtime allocation status", job: Job{Log: "CUDA runtime returned cudaErrorMemoryAllocation"}, want: "CUDA/GPU memory exhausted"},
		{name: "CUDA driver out of memory status", job: Job{Log: "CUDA_ERROR_OUT_OF_MEMORY"}, want: "CUDA/GPU memory exhausted"},
		{name: "GPU unavailable", job: Job{Log: "NVIDIA-SMI has failed because it couldn't communicate"}, want: "CUDA/GPU unavailable"},
		{name: "PyTorch driver missing", job: Job{Log: "RuntimeError: Found no NVIDIA driver on your system"}, want: "CUDA/GPU unavailable"},
		{name: "PyTorch driver too old", job: Job{Log: "The NVIDIA driver on your system is too old"}, want: "CUDA/GPU unavailable"},
		{name: "CUDA insufficient driver status", job: Job{Log: "CUDA_ERROR_INSUFFICIENT_DRIVER"}, want: "CUDA/GPU unavailable"},
		{name: "CUDA illegal memory access", job: Job{Log: "RuntimeError: CUDA error: an illegal memory access was encountered"}, want: "CUDA illegal memory access"},
		{name: "CUDA illegal address status", job: Job{Log: "cudaErrorIllegalAddress"}, want: "CUDA illegal memory access"},
		{name: "CUDA driver illegal address status", job: Job{Log: "CUDA_ERROR_ILLEGAL_ADDRESS"}, want: "CUDA illegal memory access"},
		{name: "CUDA invalid launch configuration", job: Job{Log: "CUDA error: invalid configuration argument"}, want: "CUDA kernel launch/configuration error"},
		{name: "CUDA launch out of resources", job: Job{Log: "cudaErrorLaunchOutOfResources"}, want: "CUDA kernel launch/configuration error"},
		{name: "CUDA kernel image unavailable", job: Job{Log: "CUDA error: no kernel image is available for execution on the device"}, want: "CUDA kernel image/device incompatibility"},
		{name: "CUDA invalid device ordinal", job: Job{Log: "CUDA error: invalid device ordinal"}, want: "CUDA device selection/configuration error"},
		{name: "CUDA tensor device mismatch", job: Job{Log: "Expected all tensors to be on the same device, but found at least two devices, cuda:0 and cpu!"}, want: "CUDA device selection/configuration error"},
		{name: "CUDA peer access unsupported", job: Job{Log: "CUDA error: peer access is not supported between these two devices"}, want: "CUDA peer-to-peer access unavailable"},
		{name: "Docker daemon unavailable", job: Job{Log: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"}, want: "Docker daemon unavailable"},
		{name: "Docker image denied", job: Job{Log: "Error response from daemon: pull access denied for private/image, repository does not exist or may require 'docker login'"}, want: "Container image unavailable or pull failed"},
		{name: "Docker manifest unknown", job: Job{Log: "manifest unknown: manifest tagged by example:missing"}, want: "Container image unavailable or pull failed"},
		{name: "Apptainer pull failure", job: Job{Log: "FATAL:   While pulling library image: failed to get manifest for docker://example/image:latest"}, want: "Container image unavailable or pull failed"},
		{name: "Docker OCI create failure", job: Job{Log: "docker: Error response from daemon: OCI runtime create failed: runc create failed: error during container init"}, want: "Container creation or mount failed"},
		{name: "Singularity bind mount failure", job: Job{Log: "FATAL:   container creation failed: mount hook function failure: /scratch/data->/data"}, want: "Container creation or mount failed"},
		{name: "Docker GPU runtime unavailable", job: Job{Log: "could not select device driver \"\" with capabilities: [[gpu]]"}, want: "Container GPU runtime unavailable"},
		{name: "NVIDIA container toolkit initialization", job: Job{Log: "nvidia-container-cli: initialization error: nvml error: driver not loaded"}, want: "Container GPU runtime unavailable"},
		{name: "Slurm memory limit", job: Job{Log: "slurmstepd: error: Detected 1 oom-kill event(s) in StepId=1"}, want: "Slurm memory limit exceeded"},
		{name: "rotari timeout log line", job: Job{Log: "step 3\nrotari: job timed out after 5s\nTerminated"}, want: "Job timeout reached"},
		{name: "rotari timeout error", job: Job{Error: "timed out after 1h30m0s"}, want: "Job timeout reached"},
		{name: "Slurm time limit", job: Job{Log: "slurmstepd: error: CANCELLED AT 2026-09-21 DUE TO TIME LIMIT"}, want: "Slurm time limit exceeded"},
		{name: "PBS walltime", job: Job{Log: "PBS: job exceeded walltime limit"}, want: "PBS resource or walltime limit exceeded"},
		{name: "LSF memory limit", job: Job{Log: "Exited with exit code 137. TERM_MEMLIMIT"}, want: "LSF memory limit exceeded"},
		{name: "LSF run limit", job: Job{Log: "TERM_RUNLIMIT: job killed after run limit"}, want: "LSF run time limit exceeded"},
		{name: "scheduler submission", job: Job{Log: "sbatch: error: Unable to contact slurm controller"}, want: "Scheduler submission failed"},
		{name: "invalid scheduler resource", job: Job{Log: "sbatch: error: Invalid qos specification"}, want: "Invalid scheduler resource request"},
		{name: "scheduler cancelled", job: Job{Log: "slurmstepd: error: CANCELLED AT 2026-09-21 DUE TO PREEMPTION"}, want: "Scheduler cancelled job"},
		{name: "host OOM", job: Job{Log: "Memory cgroup out of memory: Killed process 42"}, want: "Host memory exhausted"},
		{name: "killed", job: Job{Log: "Killed"}, want: "Process killed"},
		{name: "kernel fault", job: Job{Log: "BUG: unable to handle kernel NULL pointer dereference"}, want: "Kernel panic or kernel fault"},
		{name: "application panic", job: Job{Log: "panic: unexpected nil pointer"}, want: "Application panic"},
		{name: "Rust panic", job: Job{Log: "thread 'main' panicked at src/main.rs:4:5: explicit panic"}, want: "Application panic"},
		{name: "JVM uncaught exception", job: Job{Log: "Exception in thread \"main\" java.lang.IllegalStateException: invalid state"}, want: "Unhandled application exception"},
		{name: "Node.js unhandled rejection", job: Job{Log: "UnhandledPromiseRejection: request failed"}, want: "Unhandled application exception"},
		{name: "C++ uncaught exception", job: Job{Log: "terminate called after throwing an instance of 'std::runtime_error'"}, want: "Unhandled application exception"},
		{name: "JVM out of memory", job: Job{Log: "java.lang.OutOfMemoryError: Java heap space"}, want: "Runtime memory exhausted"},
		{name: "Node.js heap exhausted", job: Job{Log: "FATAL ERROR: Reached heap limit Allocation failed - JavaScript heap out of memory"}, want: "Runtime memory exhausted"},
		{name: "Rust allocation failed", job: Job{Log: "memory allocation of 1048576 bytes failed"}, want: "Runtime memory exhausted"},
		{name: "JVM stack overflow", job: Job{Log: "java.lang.StackOverflowError"}, want: "Stack overflow"},
		{name: "Node.js stack overflow", job: Job{Log: "RangeError: Maximum call stack size exceeded"}, want: "Stack overflow"},
		{name: "Go stack overflow", job: Job{Log: "runtime: goroutine stack exceeds 1000000000-byte limit\nfatal error: stack overflow"}, want: "Stack overflow"},
		{name: "Node.js missing module", job: Job{Log: "Error: Cannot find module 'left-pad'"}, want: "Application dependency or module missing"},
		{name: "JVM missing class", job: Job{Log: "java.lang.ClassNotFoundException: com.example.Main"}, want: "Application dependency or module missing"},
		{name: "JVM missing class definition", job: Job{Log: "java.lang.NoClassDefFoundError: com/example/Library"}, want: "Application dependency or module missing"},
		{name: "Go missing module", job: Job{Log: "go: no required module provides package example.com/missing"}, want: "Application dependency or module missing"},
		{name: "Rust missing crate", job: Job{Log: "error: failed to resolve: use of undeclared crate or module `serde`"}, want: "Application dependency or module missing"},
		{name: "native undefined reference", job: Job{Log: "main.cpp:(.text+0x10): undefined reference to `missing_symbol'"}, want: "Native linking failed"},
		{name: "native linker missing library", job: Job{Log: "/usr/bin/ld: cannot find -lmissing"}, want: "Native linking failed"},
		{name: "segmentation fault", job: Job{Log: "Fatal Python error: Segmentation fault"}, want: "Segmentation fault"},
		{name: "GPU Xid", job: Job{Log: "NVRM: Xid (PCI:0000:01:00): 79, GPU has fallen off the bus."}, want: "NVIDIA GPU driver/device error"},
		{name: "CUDA assertion", job: Job{Log: "RuntimeError: CUDA error: device-side assert triggered"}, want: "CUDA device-side assert"},
		{name: "NCCL", job: Job{Log: "NCCL WARN unhandled system error"}, want: "NCCL failure"},
		{name: "MPI", job: Job{Log: "MPI_ABORT was invoked on rank 0"}, want: "MPI runtime failure"},
		{name: "disk quota", job: Job{Log: "write: Disk quota exceeded"}, want: "Disk space or quota exhausted"},
		{name: "storage I/O", job: Job{Log: "cp: error writing 'checkpoint': Input/output error"}, want: "Storage device I/O error"},
		{name: "read-only filesystem", job: Job{Log: "open output: read-only file system"}, want: "Read-only filesystem"},
		{name: "missing file", job: Job{Log: "open input.json: no such file or directory"}, want: "File or directory not found"},
		{name: "permission denied", job: Job{Log: "./train: Permission denied"}, want: "Permission denied"},
		{name: "missing command", job: Job{Log: "python: command not found"}, want: "Command or executable not found"},
		{name: "bash syntax error", job: Job{Log: "./run.sh: line 4: syntax error near unexpected token `fi'"}, want: "Shell syntax error"},
		{name: "bash unterminated quote", job: Job{Log: "run.sh: line 7: unexpected EOF while looking for matching `\"'"}, want: "Shell syntax error"},
		{name: "dash syntax error", job: Job{Log: "./run.sh: 5: Syntax error: \"then\" unexpected"}, want: "Shell syntax error"},
		{name: "bash nounset", job: Job{Log: "./run.sh: line 12: OPTIONAL_DIR: unbound variable"}, want: "Shell unset variable under nounset"},
		{name: "dash nounset", job: Job{Log: "./run.sh: 8: OPTIONAL_DIR: parameter not set"}, want: "Shell unset variable under nounset"},
		{name: "shared library ABI", job: Job{Log: "ImportError: libstdc++.so.6: version `GLIBCXX_3.4.30' not found"}, want: "Shared library or ABI mismatch"},
		{name: "Python import", job: Job{Log: "ModuleNotFoundError: No module named 'torch'"}, want: "Python import or module missing"},
		{name: "Python dependency", job: Job{Log: "package-a requires package-b but version 1 is installed"}, want: "Python dependency or version conflict"},
		{name: "Python syntax", job: Job{Log: "SyntaxError: invalid syntax"}, want: "Python syntax or indentation error"},
		{name: "Python attribute", job: Job{Log: "AttributeError: 'NoneType' object has no attribute 'shape'"}, want: "Python missing name or attribute"},
		{name: "Python type", job: Job{Log: "TypeError: expected str, got None"}, want: "Python type or value error"},
		{name: "Python key", job: Job{Log: "KeyError: 'learning_rate'"}, want: "Python key or index error"},
		{name: "Python assertion", job: Job{Log: "AssertionError: invalid dataset"}, want: "Python assertion failed"},
		{name: "Python memory", job: Job{Log: "MemoryError"}, want: "Python memory error"},
		{name: "Python recursion", job: Job{Log: "RecursionError: maximum recursion depth exceeded"}, want: "Python recursion limit exceeded"},
		{name: "file descriptors", job: Job{Log: "open: Too many open files"}, want: "File descriptor limit exceeded"},
		{name: "process limit", job: Job{Log: "fork: retry: Resource temporarily unavailable"}, want: "Process or thread limit exceeded"},
		{name: "DNS", job: Job{Log: "curl: (6) Could not resolve host: example.test"}, want: "DNS lookup failed"},
		{name: "connection refused", job: Job{Log: "dial tcp: connection refused"}, want: "Network connection refused"},
		{name: "network timeout", job: Job{Error: "dial tcp: i/o timeout"}, want: "Network connection timed out"},
		{name: "connection reset", job: Job{Log: "read: connection reset by peer"}, want: "Network connection reset or closed"},
		{name: "TLS certificate", job: Job{Log: "x509: certificate signed by unknown authority"}, want: "TLS certificate or handshake failure"},
		{name: "HTTP authorization", job: Job{Log: "HTTP 403 Forbidden"}, want: "HTTP authentication or authorization failed"},
		{name: "HTTP rate limit", job: Job{Log: "request failed: status code 429"}, want: "HTTP rate limited"},
		{name: "HTTP unavailable", job: Job{Log: "503 Service Unavailable"}, want: "HTTP service unavailable"},
		{name: "HTTP gateway error", job: Job{Log: "HTTP 502 Bad Gateway"}, want: "HTTP server or bad gateway error"},
		{name: "HTTP gateway timeout", job: Job{Log: "504 Gateway Timeout"}, want: "HTTP gateway timeout"},
		{name: "SSH key", job: Job{Log: "Permission denied (publickey)."}, want: "SSH authentication or host verification failed"},
		{name: "HTTP status line", job: Job{Log: "< HTTP/1.1 500 Internal Server Error"}, want: "HTTP server or bad gateway error"},
		{name: "Python HTTPError", job: Job{Log: "urllib.error.HTTPError: HTTP Error 429: Too Many Requests"}, want: "HTTP rate limited"},
		{name: "requests HTTPError", job: Job{Log: "403 Client Error: Forbidden for url: https://example.test/api"}, want: "HTTP authentication or authorization failed"},
		{name: "NCCL warning with failure", job: Job{Log: "node1:42 NCCL WARN NET/IB : Got completion with error 12"}, want: "NCCL failure"},
		{name: "kernel oops", job: Job{Log: "[1234.5] Oops: 0002 [#1] SMP PTI"}, want: "Kernel panic or kernel fault"},
		{name: "bash killed command", job: Job{Log: "/bin/bash: line 1: 12345 Killed                  python train.py"}, want: "Process killed"},
		{name: "job control killed", job: Job{Log: "[1]+  Killed                  python train.py"}, want: "Process killed"},
		{name: "NumPy allocation", job: Job{Log: "numpy.core._exceptions._ArrayMemoryError: Unable to allocate 3.4 GiB for an array"}, want: "Python memory error"},
		{name: "glibc version", job: Job{Log: "python: /lib64/libc.so.6: version `GLIBC_2.34' not found (required by python)"}, want: "Shared library or ABI mismatch"},
		{name: "glibc version in Python import", job: Job{Log: "ImportError: /lib64/libc.so.6: version `GLIBC_2.34' not found"}, want: "Shared library or ABI mismatch"},
		{name: "NVML driver mismatch", job: Job{Log: "Failed to initialize NVML: Driver/library version mismatch"}, want: "CUDA/GPU unavailable"},
		{name: "srun allocation", job: Job{Log: "srun: error: Unable to allocate resources: Requested node configuration is not available"}, want: "Scheduler submission failed"},
		{name: "srun step", job: Job{Log: "srun: error: Unable to create step for job 123: Memory required by task is not available"}, want: "Scheduler submission failed"},
		{name: "salloc submission", job: Job{Log: "salloc: error: Job submit/allocate failed: Requested node configuration is not available"}, want: "Scheduler submission failed"},
		{name: "srun invalid account", job: Job{Log: "srun: error: Unable to allocate resources: Invalid account or account/partition combination specified"}, want: "Invalid scheduler resource request"},
		{name: "srun task OOM", job: Job{Log: "srun: error: node01: task 0: Out Of Memory"}, want: "Slurm memory limit exceeded"},
		{name: "Slurm node fail state", job: Job{Log: "JobState=NODE_FAIL Reason=NodeDown"}, want: "Slurm node failure"},
		{name: "Slurm boot fail state", job: Job{Log: "State=BOOT_FAIL"}, want: "Slurm node failure"},
		{name: "Slurm cancelled by node failure", job: Job{Log: "slurmstepd: error: *** JOB 123 ON node01 CANCELLED AT 2026-01-01T00:00:00 DUE TO NODE FAILURE, SEE SLURMCTLD LOG FOR DETAILS ***"}, want: "Slurm node failure"},
		{name: "PBS memory limit", job: Job{Log: "=>> PBS: job killed: mem 1024kb exceeded limit 512kb"}, want: "PBS resource or walltime limit exceeded"},
		{name: "LSF swap limit", job: Job{Log: "TERM_SWAP: job killed after reaching LSF swap usage limit"}, want: "LSF memory limit exceeded"},
		{name: "Node.js undefined property", job: Job{Log: "TypeError: Cannot read properties of undefined (reading 'x')"}, want: "JavaScript runtime error"},
		{name: "Node.js old undefined property", job: Job{Log: "TypeError: Cannot read property 'x' of undefined"}, want: "JavaScript runtime error"},
		{name: "Node.js not a function", job: Job{Log: "TypeError: fetchData is not a function"}, want: "JavaScript runtime error"},
		{name: "Node.js unexpected token", job: Job{Log: "SyntaxError: Unexpected token ) in JSON at position 3"}, want: "JavaScript runtime error"},
		{name: "Node.js reference error", job: Job{Log: "ReferenceError: config is not defined"}, want: "JavaScript runtime error"},
		{name: "aborted", job: Job{Log: "Aborted (core dumped)"}, want: "Process aborted"},
		{name: "bash aborted command", job: Job{Log: "./run.sh: line 3: 4242 Aborted                 (core dumped) ./train"}, want: "Process aborted"},
		{name: "double free", job: Job{Log: "double free or corruption (out)"}, want: "Native heap or stack corruption"},
		{name: "invalid free", job: Job{Log: "free(): invalid pointer"}, want: "Native heap or stack corruption"},
		{name: "stack smashing", job: Job{Log: "*** stack smashing detected ***: terminated"}, want: "Native heap or stack corruption"},
		{name: "bus error", job: Job{Log: "Bus error (core dumped)"}, want: "Bus error"},
		{name: "illegal instruction", job: Job{Log: "Illegal instruction (core dumped)"}, want: "Illegal instruction"},
		{name: "floating point exception", job: Job{Log: "Floating point exception (core dumped)"}, want: "Floating point exception"},
		{name: "cannot allocate memory", job: Job{Log: "OSError: [Errno 12] Cannot allocate memory"}, want: "Runtime memory exhausted"},
		{name: "bad_alloc", job: Job{Log: "terminate called after throwing an instance of 'std::bad_alloc'"}, want: "Runtime memory exhausted"},
		{name: "PyTorch CPU allocator", job: Job{Log: "RuntimeError: [enforce fail at alloc_cpu.cpp:83] DefaultCPUAllocator: can't allocate memory"}, want: "Runtime memory exhausted"},
		{name: "TensorFlow OOM", job: Job{Log: "tensorflow.python.framework.errors_impl.ResourceExhaustedError: OOM when allocating tensor with shape"}, want: "CUDA/GPU memory exhausted"},
		{name: "JAX OOM", job: Job{Log: "jaxlib.xla_extension.XlaRuntimeError: RESOURCE_EXHAUSTED: Out of memory while trying to allocate 1024 bytes"}, want: "CUDA/GPU memory exhausted"},
		{name: "MPS OOM", job: Job{Log: "RuntimeError: MPS backend out of memory"}, want: "CUDA/GPU memory exhausted"},
		{name: "no route", job: Job{Log: "ssh: connect to host node01 port 22: No route to host"}, want: "Network unreachable"},
		{name: "network unreachable", job: Job{Log: "OSError: [Errno 101] Network is unreachable"}, want: "Network unreachable"},
		{name: "Python name resolution", job: Job{Log: "socket.gaierror: [Errno -2] Name or service not known"}, want: "DNS lookup failed"},
		{name: "Node.js name resolution", job: Job{Log: "Error: getaddrinfo ENOTFOUND example.test"}, want: "DNS lookup failed"},
		{name: "SSH closed", job: Job{Log: "Connection closed by 10.0.0.1 port 22"}, want: "Network connection reset or closed"},
		{name: "SSH closed during exchange", job: Job{Log: "kex_exchange_identification: Connection closed by remote host"}, want: "Network connection reset or closed"},
		{name: "stale file handle", job: Job{Log: "OSError: [Errno 116] Stale file handle"}, want: "Network filesystem or mount disconnected"},
		{name: "disconnected FUSE mount", job: Job{Log: "ls: cannot access '/mnt/data': Transport endpoint is not connected"}, want: "Network filesystem or mount disconnected"},
		{name: "NFS server down", job: Job{Log: "nfs: server filer01 not responding, still trying"}, want: "Network filesystem or mount disconnected"},
		{name: "SGE enforced limit", job: Job{Log: "failed 37 : qmaster enforced h_rt, h_cpu, or h_vmem limit"}, want: "SGE resource limit exceeded"},
		{name: "SGE memory limit", job: Job{Log: "job 123 exceeds job hard limit \"h_vmem\" of queue \"all.q@node01\""}, want: "SGE resource limit exceeded"},
		{name: "SGE rejected job", job: Job{Error: "qsub: exit status 1: Unable to run job: job rejected: the requested queue does not exist."}, want: "Scheduler submission failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnoses := DiagnoseDefault(test.job)
			if len(diagnoses) != 1 || diagnoses[0].Name != test.want {
				t.Fatalf("DiagnoseDefault() = %#v, want %q", diagnoses, test.want)
			}
		})
	}
}

func TestDiagnoseDefaultIgnoresLookalikeText(t *testing.T) {
	tests := []struct {
		name string
		log  string
	}{
		{name: "status code in URL", log: "Downloading https://example.test/model-00500-of-01000.safetensors"},
		{name: "status code in HTTP URL path", log: "GET http://example.test/items/403 completed"},
		{name: "step counter", log: "step 429 loss 0.12 lr 5e-4"},
		{name: "informational NCCL warning", log: "node1:42 NCCL WARN NET/IB : No device found."},
		{name: "signal number prefix", log: "received signal 110 from peer"},
		{name: "application oops message", log: "Oops: something went wrong, retrying"},
		{name: "ordinary mention of missing module", log: "The documentation explains how to report a missing module."},
		{name: "ordinary allocation message", log: "memory allocation of 1048576 bytes succeeded"},
		{name: "ordinary Docker mention", log: "The Docker daemon is running and the container completed successfully."},
		{name: "ordinary image manifest discussion", log: "The release notes describe an unknown manifest format."},
		{name: "ordinary application mount failure", log: "The application could not mount the requested dataset in its own virtual filesystem."},
		{name: "ordinary GPU capability discussion", log: "The device driver supports GPU capabilities required by the model."},
		{name: "generic illegal memory wording", log: "The parser reported an illegal memory access in its documentation."},
		{name: "generic configuration wording", log: "configuration argument is invalid"},
		{name: "generic peer access wording", log: "peer access is not supported by this network protocol"},
		{name: "ambiguous cuDNN failure", log: "CUDNN_STATUS_EXECUTION_FAILED"},
		{name: "ordinary linker text", log: "The linker command failed yesterday, but the retry succeeded."},
		{name: "ordinary unset variable text", log: "This script enables set -u so unset parameters fail."},
		{name: "exception name inside identifier", log: "counter.valueerrors=0 keyerrorsuppressed=1"},
		{name: "errno name inside word", log: "prefix_enoentry and leacces and teagain"},
		{name: "another tool's timeout", log: "request timed out after 30s, retrying"},
		{name: "quoted rotari timeout", log: "see: rotari: job timed out after 5s in the docs"},
		{name: "ordinary killed count", log: "the supervisor killed 3 workers"},
		{name: "srun task exit", log: "srun: error: node01: task 0: Exited with exit code 1"},
		{name: "truncated archive", log: "tar: Unexpected EOF in archive"},
		{name: "version warning", log: "warning: incompatible version of foo, continuing"},
		{name: "ordinary aborted text", log: "transaction aborted, retrying"},
		{name: "ordinary bus error text", log: "the event bus error counter is 0"},
		{name: "ordinary connection closed", log: "connection closed by client, reconnecting"},
		{name: "ordinary free call", log: "free(ptr) returned"},
		{name: "dependency warning", log: "WARNING: package-a requires package-b but version 1 is installed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnoses := DiagnoseDefault(Job{Log: test.log}); len(diagnoses) != 0 {
				t.Fatalf("DiagnoseDefault() = %#v, want no diagnosis", diagnoses)
			}
		})
	}
}

func TestDiagnoseDefaultDoesNotGuess(t *testing.T) {
	if diagnoses := DiagnoseDefault(Job{Log: "the job failed"}); len(diagnoses) != 0 {
		t.Fatalf("DiagnoseDefault() = %#v, want no diagnosis", diagnoses)
	}
}

func TestDiagnoseDefaultReportsSpecificPythonExceptionOnce(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want string
	}{
		{name: "value error", log: "Traceback (most recent call last):\n  File \"train.py\", line 10\nValueError: invalid batch size", want: "Python type or value error"},
		{name: "CUDA OOM is not a Python memory error", log: "Traceback (most recent call last):\ntorch.cuda.OutOfMemoryError: CUDA out of memory. Tried to allocate 2.00 GiB", want: "CUDA/GPU memory exhausted"},
		{name: "missing file", log: "Traceback (most recent call last):\nFileNotFoundError: [Errno 2] No such file or directory: 'x'", want: "File or directory not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnoses := DiagnoseDefault(Job{Log: test.log})
			if len(diagnoses) != 1 || diagnoses[0].Name != test.want {
				t.Fatalf("DiagnoseDefault() = %#v, want only %q", diagnoses, test.want)
			}
		})
	}
}

func TestDiagnoseDefaultFallsBackToFinalPythonException(t *testing.T) {
	log := "Traceback (most recent call last):\n  File \"a.py\", line 1\nRuntimeError: first\nTraceback (most recent call last):\n  File \"a.py\", line 2\nmypkg.CustomError: final failure"
	diagnoses := DiagnoseDefault(Job{Log: log})
	if len(diagnoses) != 1 || diagnoses[0].Name != "Python exception" || diagnoses[0].Evidence != "mypkg.CustomError: final failure" {
		t.Fatalf("DiagnoseDefault() = %#v, want final Python exception", diagnoses)
	}
}

func TestDiagnoseDefaultOrdersLatestEvidenceFirst(t *testing.T) {
	log := "open /data/in.json: no such file or directory\nread: connection reset by peer\nretrying download\nwrite ckpt: No space left on device"
	diagnoses := DiagnoseDefault(Job{Log: log, Error: "sbatch: error: Batch job submission failed"})
	want := []string{"Scheduler submission failed", "Disk space or quota exhausted", "Network connection reset or closed", "File or directory not found"}
	if len(diagnoses) != len(want) {
		t.Fatalf("DiagnoseDefault() = %#v, want %q", diagnoses, want)
	}
	for index, name := range want {
		if diagnoses[index].Name != name {
			t.Fatalf("DiagnoseDefault()[%d] = %q, want %q (all: %#v)", index, diagnoses[index].Name, name, diagnoses)
		}
	}
}

func TestDiagnoseDefaultCitesLatestMatchingLine(t *testing.T) {
	diagnoses := DiagnoseDefault(Job{Log: "dial tcp 10.0.0.1:80: connection refused\nretrying\ndial tcp 10.0.0.2:80: connection refused"})
	if len(diagnoses) != 1 || diagnoses[0].Evidence != "dial tcp 10.0.0.2:80: connection refused" {
		t.Fatalf("DiagnoseDefault() = %#v, want latest matching line as evidence", diagnoses)
	}
}

func TestDefaultRuleIDsAreUnique(t *testing.T) {
	seen := make(map[string]string)
	for _, rule := range DefaultRules() {
		id := RuleID(rule)
		if id == "" {
			t.Fatalf("rule %q has an empty ID", rule.Name)
		}
		if previous, ok := seen[id]; ok {
			t.Fatalf("rules %q and %q share ID %q", previous, rule.Name, id)
		}
		seen[id] = rule.Name
	}
}

func TestResolveAndMatchRuleSelectors(t *testing.T) {
	if err := ResolveRuleSelectors([]string{"cuda-gpu-memory-exhausted", "python IMPORT"}); err != nil {
		t.Fatal(err)
	}
	if err := ResolveRuleSelectors([]string{"not-a-diagnosis"}); err == nil {
		t.Fatal("unknown diagnosis was accepted")
	}
	exitCode := 1
	if !MatchesRuleSelectors([]string{"cuda-gpu-memory-exhausted"}, Job{ExitCode: &exitCode, Log: "CUDA out of memory"}) {
		t.Fatal("diagnosis slug did not match current rules")
	}
	if !MatchesRuleSelectors([]string{strings.ToLower("memory exhausted")}, Job{ExitCode: &exitCode, Log: "CUDA out of memory"}) {
		t.Fatal("diagnosis name substring did not match")
	}
}
