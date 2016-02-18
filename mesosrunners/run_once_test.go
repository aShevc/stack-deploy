/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package mesosrunners

import (
	"testing"

	"github.com/elodina/stack-deploy/framework"
	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRunOnceRunner(t *testing.T) {
	Convey("Application ID from Task ID", t, func() {
		Convey("should extract proper application ID", func() {
			So(applicationIDFromTaskID("foobar|ip-123-123-123-123|f81d4fae-7dec-11d0-a765-00a0c91e6bf6"), ShouldEqual, "foobar")
			So(func() { applicationIDFromTaskID("foobar-ip-123-123-123-123-f81d4fae-7dec-11d0-a765-00a0c91e6bf6") }, ShouldPanic)
		})
	})

	Convey("Hostname from Task ID", t, func() {
		Convey("should extract proper hostname", func() {
			So(hostnameFromTaskID("foobar|ip-123-123-123-123|f81d4fae-7dec-11d0-a765-00a0c91e6bf6"), ShouldEqual, "ip-123-123-123-123")
			So(func() { hostnameFromTaskID("foobar-ip-123-123-123-123-f81d4fae-7dec-11d0-a765-00a0c91e6bf6") }, ShouldPanic)
			So(func() { hostnameFromTaskID("foobar|ip-123-123-123-123-f81d4fae-7dec-11d0-a765-00a0c91e6bf6") }, ShouldPanic)
		})
	})

	Convey("Run once task runner", t, func() {
		runner := NewRunOnceRunner()

		Convey("should decline offers if no applications are staged", func() {
			declineReason, err := runner.ResourceOffer(nil, nil)

			So(declineReason, ShouldEqual, "all tasks are running")
			So(err, ShouldBeNil)
		})

		Convey("should stage applications properly", func() {
			So(runner.applications, ShouldHaveLength, 0)

			application := &framework.Application{
				Type:          "foo",
				ID:            "foo",
				Cpu:           0.5,
				Mem:           512,
				Instances:     "3",
				LaunchCommand: "sleep 10",
			}

			statusChan := runner.StageApplication(application)

			So(statusChan, ShouldNotBeNil)
			So(runner.applications["foo"], ShouldNotBeNil)
			So(runner.applications["foo"].InstancesLeftToRun, ShouldEqual, 3)
			So(runner.applications["foo"].Application, ShouldEqual, application)
		})
	})
}

func TestRunOnceApplicationContext(t *testing.T) {
	Convey("Run once application context", t, func() {
		Convey("should decline offer", func() {
			Convey("if all instances are already running", func() {
				ctx := NewRunOnceApplicationContext()
				ctx.InstancesLeftToRun = 0
				So(ctx.Matches(nil), ShouldEqual, "all instances are staged/running")
			})

			Convey("if application is already staged on given host", func() {
				ctx := NewRunOnceApplicationContext()
				ctx.InstancesLeftToRun = 1
				ctx.stagedInstances["slave0"] = mesos.TaskState_TASK_STAGING

				So(ctx.Matches(&mesos.Offer{
					Hostname: proto.String("slave0"),
				}), ShouldContainSubstring, "application instance is already staged/running on host")
			})

			Convey("if it does not have enough CPU", func() {
				ctx := NewRunOnceApplicationContext()
				ctx.InstancesLeftToRun = 1
				ctx.Application = &framework.Application{
					Type:          "foo",
					ID:            "foo",
					Cpu:           0.5,
					Mem:           512,
					Instances:     "3",
					LaunchCommand: "sleep 10",
				}

				So(ctx.Matches(&mesos.Offer{
					Hostname: proto.String("slave0"),
				}), ShouldEqual, "no cpus")
			})

			Convey("if it does not have enough memory", func() {
				ctx := NewRunOnceApplicationContext()
				ctx.InstancesLeftToRun = 1
				ctx.Application = &framework.Application{
					Type:          "foo",
					ID:            "foo",
					Cpu:           0.5,
					Mem:           512,
					Instances:     "3",
					LaunchCommand: "sleep 10",
				}

				So(ctx.Matches(&mesos.Offer{
					Hostname: proto.String("slave0"),
					Resources: []*mesos.Resource{
						util.NewScalarResource("cpus", 1.5),
					},
				}), ShouldEqual, "no mem")
			})
		})

		Convey("should accept offer if it matches", func() {
			ctx := NewRunOnceApplicationContext()
			ctx.InstancesLeftToRun = 1
			ctx.Application = &framework.Application{
				Type:          "foo",
				ID:            "foo",
				Cpu:           0.5,
				Mem:           512,
				Instances:     "3",
				LaunchCommand: "sleep 10",
			}

			So(ctx.Matches(&mesos.Offer{
				Hostname: proto.String("slave0"),
				Resources: []*mesos.Resource{
					util.NewScalarResource("cpus", 1.5),
					util.NewScalarResource("mem", 2048),
				},
			}), ShouldEqual, "")
		})

		Convey("should build a correct TaskInfo", func() {
			ctx := NewRunOnceApplicationContext()
			ctx.Application = &framework.Application{
				Type:          "foo",
				ID:            "foo",
				Cpu:           0.5,
				Mem:           512,
				Instances:     "3",
				LaunchCommand: "sleep 10",
			}

			offer := &mesos.Offer{
				Hostname: proto.String("slave0"),
				Resources: []*mesos.Resource{
					util.NewScalarResource("cpus", 1.5),
					util.NewScalarResource("mem", 2048),
				},
			}

			taskInfo := ctx.newTaskInfo(offer)
			So(taskInfo.GetName(), ShouldEqual, "foo.slave0")
			So(taskInfo.GetTaskId().GetValue(), ShouldContainSubstring, "foo|slave0|")
			So(taskInfo.GetCommand(), ShouldNotBeNil)
			So(taskInfo.GetCommand().GetValue(), ShouldEqual, "sleep 10")
			So(taskInfo.GetCommand().GetShell(), ShouldBeTrue)
		})

		Convey("should not consider all tasks finished", func() {
			ctx := NewRunOnceApplicationContext()
			ctx.Application = &framework.Application{
				Type:          "foo",
				ID:            "foo",
				Cpu:           0.5,
				Mem:           512,
				Instances:     "3",
				LaunchCommand: "sleep 10",
			}

			Convey("if there are instances not yet staged", func() {
				ctx.InstancesLeftToRun = 1
				So(ctx.allTasksFinished(), ShouldBeFalse)
			})

			Convey("if there are instances not yet finished", func() {
				ctx.InstancesLeftToRun = 0
				ctx.stagedInstances["slave0"] = mesos.TaskState_TASK_RUNNING
				So(ctx.allTasksFinished(), ShouldBeFalse)
			})
		})

		Convey("should consider all tasks finished", func() {
			ctx := NewRunOnceApplicationContext()
			ctx.Application = &framework.Application{
				Type:          "foo",
				ID:            "foo",
				Cpu:           0.5,
				Mem:           512,
				Instances:     "3",
				LaunchCommand: "sleep 10",
			}

			Convey("if there was nothing to stage at all", func() {
				So(ctx.allTasksFinished(), ShouldBeTrue)
			})

			Convey("if all tasks are in state finished and no instances left to run", func() {
				ctx.stagedInstances["slave0"] = mesos.TaskState_TASK_FINISHED
				So(ctx.allTasksFinished(), ShouldBeTrue)
				So(ctx.InstancesLeftToRun, ShouldEqual, 0)
			})
		})
	})
}
