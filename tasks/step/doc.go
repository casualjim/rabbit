//Package step contains the steps to do workflow orchestration.
//
// These steps are units of work that have a Run/Rollback method.
// When a step returns a PermanentError then the entire batch of work will be rolled back.
// When a step returns a TransientError then the batch con continue.
// When a step return a different error it will roll back that single step and use the return value
// of rollback as return value for the current run.
//
// Steps are exeucted by an executor, there is one executor consuming types can make use of.
// There is a builder for running a step
package step
