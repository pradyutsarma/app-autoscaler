package integration_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	. "integration"
	"io/ioutil"
	"net/http"
	"regexp"
)

var _ = Describe("Integration_Broker_Api", func() {

	var (
		regPath = regexp.MustCompile(`^/v2/schedules/.*$`)

		serviceInstanceId       string
		bindingId               string
		orgId                   string
		spaceId                 string
		appId                   string
		schedulePolicyJson      []byte
		invalidSchemaPolicyJson []byte
		invalidDataPolicyJson   []byte
	)

	BeforeEach(func() {
		initializeHttpClient("servicebroker.crt", "servicebroker.key", "autoscaler-ca.crt", brokerApiHttpRequestTimeout)

		fakeScheduler = ghttp.NewServer()
		apiServerConfPath = components.PrepareApiServerConfig(components.Ports[APIServer], components.Ports[APIPublicServer], dbUrl, fakeScheduler.URL(), fmt.Sprintf("https://127.0.0.1:%d", components.Ports[ScalingEngine]), fmt.Sprintf("https://127.0.0.1:%d", components.Ports[MetricsCollector]), tmpDir)
		serviceBrokerConfPath = components.PrepareServiceBrokerConfig(components.Ports[ServiceBroker], brokerUserName, brokerPassword, dbUrl, fmt.Sprintf("https://127.0.0.1:%d", components.Ports[APIServer]), brokerApiHttpRequestTimeout, tmpDir)

		startApiServer()
		startServiceBroker()
		brokerAuth = base64.StdEncoding.EncodeToString([]byte("username:password"))
		serviceInstanceId = getRandomId()
		orgId = getRandomId()
		spaceId = getRandomId()
		bindingId = getRandomId()
		appId = getRandomId()
		//add a service instance
		resp, err := provisionServiceInstance(serviceInstanceId, orgId, spaceId)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
		resp.Body.Close()

		schedulePolicyJson = readPolicyFromFile("fakePolicyWithSchedule.json")
		invalidSchemaPolicyJson = readPolicyFromFile("fakeInvalidPolicy.json")
		invalidDataPolicyJson = readPolicyFromFile("fakeInvalidDataPolicy.json")
	})

	AfterEach(func() {
		//clean the service instance added in before each
		resp, err := deprovisionServiceInstance(serviceInstanceId)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		resp.Body.Close()
		fakeScheduler.Close()
		stopAll()
	})

	Describe("Bind Service", func() {
		Context("Policy with schedules", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
				fakeScheduler.RouteToHandler("DELETE", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
			})

			AfterEach(func() {
				//clear the binding
				resp, err := unbindService(bindingId, appId, serviceInstanceId)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				resp.Body.Close()
			})

			It("creates a binding", func() {
				resp, err := bindService(bindingId, appId, serviceInstanceId, schedulePolicyJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))
				resp.Body.Close()
				Consistently(fakeScheduler.ReceivedRequests).Should(HaveLen(1))

				By("checking the API Server")
				var expected map[string]interface{}
				err = json.Unmarshal(schedulePolicyJson, &expected)
				Expect(err).NotTo(HaveOccurred())

				checkResponseContent(getPolicy, appId, http.StatusOK, expected, INTERNAL)
			})
		})

		Context("Invalid policy Schema", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
			})

			It("does not create a binding", func() {
				schedulerCount := len(fakeScheduler.ReceivedRequests())
				resp, err := bindService(bindingId, appId, serviceInstanceId, invalidSchemaPolicyJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				respBody, err := ioutil.ReadAll(resp.Body)
				Expect(string(respBody)).To(Equal(`{"error":[{"property":"instance","message":"is not any of [subschema 0],[subschema 1]","schema":"/policySchema","instance":{"instance_min_count":10,"instance_max_count":4},"name":"anyOf","argument":["[subschema 0]","[subschema 1]"],"stack":"instance is not any of [subschema 0],[subschema 1]"}]}`))
				resp.Body.Close()
				Consistently(fakeScheduler.ReceivedRequests).Should(HaveLen(schedulerCount))

				By("checking the API Server")
				resp, err = getPolicy(appId, INTERNAL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				resp.Body.Close()
			})
		})

		Context("Invalid policy Data", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
			})

			It("does not create a binding", func() {
				schedulerCount := len(fakeScheduler.ReceivedRequests())
				resp, err := bindService(bindingId, appId, serviceInstanceId, invalidDataPolicyJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				respBody, err := ioutil.ReadAll(resp.Body)
				Expect(string(respBody)).To(Equal(`{"error":[{"property":"instance.scaling_rules[0].cool_down_secs","message":"must have a minimum value of 60","schema":{"type":"number","minimum":60,"maximum":3600},"instance":-300,"name":"minimum","argument":60,"stack":"instance.scaling_rules[0].cool_down_secs must have a minimum value of 60"}]}`))
				resp.Body.Close()
				Consistently(fakeScheduler.ReceivedRequests).Should(HaveLen(schedulerCount))

				By("checking the API Server")
				resp, err = getPolicy(appId, INTERNAL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				resp.Body.Close()
			})
		})

		Context("ApiServer is down", func() {
			BeforeEach(func() {
				stopApiServer()
				_, err := getPolicy(appId, INTERNAL)
				Expect(err).To(HaveOccurred())
				fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusInternalServerError, "error"))
			})

			It("should return 500", func() {
				schedulerCount := len(fakeScheduler.ReceivedRequests())
				resp, err := bindService(bindingId, appId, serviceInstanceId, schedulePolicyJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				resp.Body.Close()
				Consistently(fakeScheduler.ReceivedRequests).Should(HaveLen(schedulerCount))
			})
		})

		Context("Scheduler returns error", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusInternalServerError, "error"))
			})

			It("should return 500", func() {
				schedulerCount := len(fakeScheduler.ReceivedRequests())
				resp, err := bindService(bindingId, appId, serviceInstanceId, schedulePolicyJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				resp.Body.Close()
				Consistently(fakeScheduler.ReceivedRequests).Should(HaveLen(schedulerCount + 1))

				By("checking the API Server")
				resp, err = getPolicy(appId, INTERNAL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				resp.Body.Close()
			})
		})
	})

	Describe("Unbind Service", func() {
		BeforeEach(func() {
			brokerAuth = base64.StdEncoding.EncodeToString([]byte("username:password"))
			//do a bind first
			fakeScheduler.RouteToHandler("PUT", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
			resp, err := bindService(bindingId, appId, serviceInstanceId, schedulePolicyJson)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			resp.Body.Close()
		})

		BeforeEach(func() {
			fakeScheduler.RouteToHandler("DELETE", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
		})

		It("should return 200", func() {
			resp, err := unbindService(bindingId, appId, serviceInstanceId)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()

			By("checking the API Server")
			resp, err = getPolicy(appId, INTERNAL)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			resp.Body.Close()
		})

		Context("Policy does not exist", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("DELETE", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
				//detach the appId's policy first
				resp, err := detachPolicy(appId, INTERNAL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				resp.Body.Close()
			})

			It("should return 200", func() {
				resp, err := unbindService(bindingId, appId, serviceInstanceId)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				resp.Body.Close()
			})
		})

		Context("APIServer is down", func() {
			BeforeEach(func() {
				stopApiServer()
				_, err := detachPolicy(appId, INTERNAL)
				Expect(err).To(HaveOccurred())
				fakeScheduler.RouteToHandler("DELETE", regPath, ghttp.RespondWith(http.StatusOK, "successful"))
			})

			It("should return 500", func() {
				resp, err := unbindService(bindingId, appId, serviceInstanceId)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				resp.Body.Close()
			})
		})

		Context("Scheduler returns error", func() {
			BeforeEach(func() {
				fakeScheduler.RouteToHandler("DELETE", regPath, ghttp.RespondWith(http.StatusInternalServerError, "error"))
			})

			It("should return 500 and not delete the binding info", func() {
				resp, err := unbindService(bindingId, appId, serviceInstanceId)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				resp.Body.Close()

				By("checking the API Server")
				resp, err = getPolicy(appId, INTERNAL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				resp.Body.Close()
			})
		})
	})
})
