# Scheduler integration coverage

This is an internal test-scope note, not a user guide, compatibility guarantee,
or production-image recommendation.

- Slurm, PBS, and SGE are integration-tested in CI on every push against a
  Slurm container, an OpenPBS container, and a Grid Engine container. Passing
  those tests does not certify compatibility with every real cluster
  configuration.
- LSF is covered by unit tests using fake scheduler commands. CI cannot test
  it against a real LSF, because LSF Community Edition must be downloaded from
  IBM by hand. With a downloaded package,
  [scripts/scheduler-integration.sh](../../../scripts/scheduler-integration.sh)
  `lsf` runs the scheduler container tests against it in a local Docker
  container. They last passed against LSF Community Edition 10.1.0.15 on
  2026-11-13.
- SGE is also covered by unit tests using fake commands. The
  [Scheduler integration workflow](../../../.github/workflows/scheduler-integration.yml)
  tests it against a digest-pinned CentOS 7 Grid Engine image last published
  in 2021. This is a compatibility smoke test, not certification for every
  Grid Engine fork or a recommendation to use that image in production.

Relevant tests include the [LSF unit tests](../../../internal/executor/lsf_test.go),
the [SGE unit tests](../../../internal/executor/sge_test.go), the
[scheduler container tests](../../../cmd/rotari/scheduler_container_test.go),
and the [container run tests](../../../cmd/rotari/scheduler_container_run_test.go).
