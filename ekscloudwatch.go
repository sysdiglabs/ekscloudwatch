package ekscloudwatch

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type EKSAuditLogs struct {
	cw              *cloudwatchlogs.CloudWatchLogs
	k8sAuditURI     string
	logGroupName    string
	pollingInterval time.Duration
}

// New creates a new instance of the CW client that periodically polls cloudwatch and sends events to the specified
// Sysdig agent URI
func New(k8sAuditURI string, clusterNameOverride string, awsRegionOverride string, pollingInterval time.Duration) (*EKSAuditLogs, error) {
	metaSession := session.Must(session.NewSession())
	metaClient := ec2metadata.New(metaSession)

	var err error

	var region string
	// if awsRegionOverride isn't set use it, otherwise try to autodetect it from the instance
	if awsRegionOverride != "" {
    		region = awsRegionOverride
	} else {
		region, err = metaClient.Region()
		if err != nil {
			log.Printf("Could not get region from EC2 metadata.")
			log.Printf("Sysdig EKS CloudWatch agent can only be run inside an EC2 instance and EC2 instance metadata should be available.")
			return nil, err
		}
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	var clusterName string
	// if clusterNameOverride isn't set use it, otherwise try to autodetect it from the instance
	if clusterNameOverride != "" {
		clusterName = clusterNameOverride
	} else {
		instanceIdentity, err := metaClient.GetInstanceIdentityDocument()
		if err != nil {
			log.Printf("Could not get EC2 instance information.")
			return nil, err
		}

		ec2client := ec2.New(sess)
		diOut, err := ec2client.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{aws.String(instanceIdentity.InstanceID)},
		})

		if err != nil {
			log.Printf("Could not query AWS for available instances and no cluster name was specified.")
			return nil, err
		}

		if len(diOut.Reservations) != 1 {
			return nil, fmt.Errorf("Expected one AWS EC2 reservation for instance %s, got %d", instanceIdentity.InstanceID, len(diOut.Reservations))
		}

		if len(diOut.Reservations[0].Instances) != 1 {
			return nil, fmt.Errorf("Expected one AWS EC2 instance that matches %s, got %d", instanceIdentity.InstanceID, len(diOut.Reservations))
		}

		instance := diOut.Reservations[0].Instances[0]

		tags := instance.Tags
		for _, tag := range tags {
			if *tag.Key == "eksctl.cluster.k8s.io/v1alpha1/cluster-name" || *tag.Key == "alpha.eksctl.io/cluster-name" {
				clusterName = *tag.Value
			}
		}

		if clusterName == "" {
			return nil, errors.New("Could not determine EKS cluster name")
		}
	}

	ret := new(EKSAuditLogs)
	ret.cw = cloudwatchlogs.New(sess)
	ret.k8sAuditURI = k8sAuditURI
	ret.pollingInterval = pollingInterval
	ret.logGroupName = fmt.Sprintf("/aws/eks/%s/cluster", clusterName)

	// Test if we can access our logs
	_, err = ret.cw.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		Limit:        aws.Int64(1),
		LogGroupName: aws.String(ret.logGroupName),
	})

	if err != nil {
		log.Printf("Could not read CloudWatch logs. Check IAM configuration.")
		return nil, err
	}

	return ret, nil
}

func (e *EKSAuditLogs) sendEvent(message string) error {
	resp, err := http.Post(
		e.k8sAuditURI, "application/json",
		bytes.NewBuffer([]byte(message)))

	if err != nil {
		log.Printf("Error while sending events to agent: %v.\n", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Error %v %v from agent: %v\n", resp.StatusCode, resp.Status, resp.Body)
		return err
	}

	return nil
}

func (e *EKSAuditLogs) sendFilteredLogs(startTime time.Time, endTime time.Time, filter string) (int, error) {
	params := &cloudwatchlogs.FilterLogEventsInput{
		StartTime:           aws.Int64(startTime.UnixNano() / 1000000), // TODO use the right duration
		EndTime:             aws.Int64(endTime.UnixNano() / 1000000),
		LogGroupName:        aws.String(e.logGroupName),
		LogStreamNamePrefix: aws.String("kube-apiserver-audit"),
		FilterPattern:       aws.String(filter),
	}

	n := 0

	err := e.cw.FilterLogEventsPages(params,
		func(page *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
			nThisPage := 0
			for _, event := range page.Events {
				e.sendEvent(*event.Message)

				nThisPage++
				n++
			}

			log.Printf("%v logs sent to the agent (%v total)\n", nThisPage, n)
			return !lastPage
		})

	return n, err
}

// RunForever continuously polls CloudWatch and sends events.
// Only returns in case of error.
func (e *EKSAuditLogs) RunForever(filter string) error {
	// Very recent events might not have been ingested yet
	ingestionDelay, _ := time.ParseDuration("15s")

	lastFetch := time.Now().Add(-e.pollingInterval)
	total := 0
	for {
		endTime := time.Now().Add(-ingestionDelay)
		n, err := e.sendFilteredLogs(lastFetch, endTime, filter)

		if err != nil {
			return err
		}

		total += n
		log.Printf("%v total logs", total)
		lastFetch = endTime
		time.Sleep(e.pollingInterval)
	}
}
