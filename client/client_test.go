package client

import (
	"fmt"
	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

const (
	configmapName      = "test"
	configmapNamespace = "default"
)

var _ = Describe("client", func() {
	Context("current env", func() {
		BeforeEach(func() {
			err := k8sDelegatedClient.DeleteAllOf(testCtx, &topolsv1.LogicalVolume{})
			Expect(err).ShouldNot(HaveOccurred())
			cm := &corev1.ConfigMap{}
			cm.Name = configmapName
			cm.Namespace = configmapNamespace
			k8sDelegatedClient.Delete(testCtx, cm)
		})

		Context("wrappedReader", func() {
			Context("Get", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setCurrentLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolsv1.LogicalVolume)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
					Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
					Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
					Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
					Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
				})
			})

			Context("List", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					lvlist := new(topolsv1.LogicalVolumeList)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err := c.List(testCtx, lvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(lvlist.Items)).Should(Equal(2))
					for i, lv := range lvlist.Items {
						Expect(lv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(lv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
						Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(lv.Spec.AccessType).Should(Equal("rw"))
						Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(lv.Status.Code).Should(Equal(codes.Unknown))
						Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					}
				})
			})
		})

		Context("wrappedClient", func() {
			Context("Get", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setCurrentLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						checklv := new(topolsv1.LogicalVolume)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						g.Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						g.Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						g.Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
						g.Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						g.Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						g.Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						g.Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						g.Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						g.Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						g.Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					}).Should(Succeed())
				})
			})

			Context("List", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					Eventually(func(g Gomega) {
						lvlist := new(topolsv1.LogicalVolumeList)
						c := NewWrappedClient(k8sDelegatedClient)
						err := c.List(testCtx, lvlist)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(len(lvlist.Items)).Should(Equal(2))
						for i, lv := range lvlist.Items {
							g.Expect(lv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
							g.Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
							g.Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							g.Expect(lv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
							g.Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							g.Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							g.Expect(lv.Spec.AccessType).Should(Equal("rw"))
							g.Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							g.Expect(lv.Status.Code).Should(Equal(codes.Unknown))
							g.Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
							g.Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}).Should(Succeed())
				})
			})

			Context("Create", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					c := NewWrappedClient(k8sDelegatedClient)
					err := c.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolsv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("Delete", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Delete(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolsv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				})
			})

			Context("Update", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv.Annotations = ann
					lv.Spec.Name = fmt.Sprintf("updated-current-%d", i)
					lv.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolsv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Annotations).Should(Equal(ann))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("Patch", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv2 := lv.DeepCopy()
					lv2.Annotations = ann
					lv2.Spec.Name = fmt.Sprintf("updated-current-%d", i)
					lv2.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					c := NewWrappedClient(k8sDelegatedClient)
					patch := client.MergeFrom(lv)
					err = c.Patch(testCtx, lv2, patch)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolsv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Annotations).Should(Equal(ann))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("DeleteAllOf", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					checklvlist := new(topolsv1.LogicalVolumeList)
					err := k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(2))

					lv := new(topolsv1.LogicalVolume)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.DeleteAllOf(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklvlist = new(topolsv1.LogicalVolumeList)
					err = k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(0))
				})
			})

			Context("SubResourceClient", func() {
				Context("Get", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Get(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})

				Context("Create", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Create(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})
			})

			Context("SubResourceWriter", func() {
				Context("Update", func() {
					It("standard case", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						checklv := new(topolsv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					})
				})

				Context("Patch", func() {
					It("standard case", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						lv2 := lv.DeepCopy()
						setCurrentLVStatus(lv2, i)
						patch := client.MergeFrom(lv)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Patch(testCtx, lv2, patch)
						Expect(err).ShouldNot(HaveOccurred())

						checklv := new(topolsv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topols.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					})
				})
			})
		})
	})
})
