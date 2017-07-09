//Package steps contains the steps to do workflow orchestration.
//
// These steps are units of work that have a Run/Rollback method.
// When a step returns a PermanentError then the entire batch of work will be rolled back.
// When a step returns a TransientError then the batch con continue.
// When a step return a different error it will roll back that single step and use the return value
// of rollback as return value for the current run.
//
//		steps.Execution(
//			steps.ParentContext(context.Background()),
//			steps.Should(rollback.Always),
//			steps.PublishTo(eventbus.New(rabbit.GoLog(os.Stderr, "", 0))),
//		).Run(
//			steps.Concurrent(
//				"provision-workers",
//				steps.If(isVsphereEnterprise).Then(
//					steps.Pipeline(
//						steps.Stateless("create-disk", createDiskFn, deleteDiskFn),
//						steps.Stateless("create-docker-disk", createDockerDiskFn, deleteDockerDiskFn),
//						steps.Stateless("create-state-disk", createStateDiskFn, deleteStateDiskFn),
//						steps.Stateless("create-state-disk", createStateDiskFn, deleteStateDiskFn),
//						steps.Retry(
//							backoff.WithMaxTries(backoff.NewConstantBackOff(1*time.Second), 4),
//							steps.Stateless("create-vm", createVMFn, deleteVMFn),
//						),
//						steps.Stateless("attach-disks", attachDisksFn, detachDisksFn),
//					)
//				).Else(
//					steps.Pipeline(
//						steps.Stateless("create-disk", createDiskFn, deleteDiskFn),
//						steps.Stateless("create-docker-disk", createDockerDiskFn, deleteDockerDiskFn),
//						steps.Stateless("create-state-disk", createStateDiskFn, deleteStateDiskFn),
//						steps.Stateless("create-state-disk", createStateDiskFn, deleteStateDiskFn),
//						steps.Retry(
//							backoff.WithMaxTries(backoff.NewConstantBackOff(1*time.Second), 4),
//							steps.Stateless("create-vm", createVMFn, deleteVMFn),
//						),
//						steps.Stateless("attach-disks", attachDisksFn, detachDisksFn),
//					)
//				)
//			)
//		)
//
// Steps are executed by an executor, there is one executor consuming types can make use of.
// There is a builder for running a step
package steps
