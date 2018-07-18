package handler

import (
	"strings"

	"github.com/sirupsen/logrus"
	helper "github.com/stakater/Reloader/internal/pkg/helper"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	configmapUpdateOnChangeAnnotation = "reloader.stakater.com/configmap.update-on-change"
	// Adding separate annotation to differentiate between configmap and secret
	secretUpdateOnChangeAnnotation = "reloader.stakater.com/secret.update-on-change"
)

// ResourceUpdatedHandler contains updated objects
type ResourceUpdatedHandler struct {
	Resource    interface{}
	OldResource interface{}
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
			rollingUpgrade(r, "configmaps", "deployments")
			rollingUpgrade(r, "configmaps", "daemonsets")
			rollingUpgrade(r, "configmaps", "statefulSets")
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
			rollingUpgrade(r, "secrets", "deployments")
			rollingUpgrade(r, "secrets", "daemonsets")
			rollingUpgrade(r, "secrets", "statefulSets")
		} else {
			logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
		}
	}
	return nil
}

func rollingUpgrade(r ResourceUpdatedHandler, resourceType string, rollingUpgradeType string) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	var namespace, name, sshData, envName string
	if resourceType == "configmaps" {
		namespace = r.Resource.(*v1.ConfigMap).Namespace
		name = r.Resource.(*v1.ConfigMap).Name
		sshData = helper.ConvertConfigmapToSHA(r.Resource.(*v1.ConfigMap))
		envName = "_CONFIGMAP"
	} else if resourceType == "secrets" {
		namespace = r.Resource.(*v1.Secret).Namespace
		name = r.Resource.(*v1.Secret).Name
		sshData = helper.ConvertSecretToSHA(r.Resource.(*v1.Secret))
		envName = "_SECRET"
	}

	if rollingUpgradeType == "deployments" {
		rollingUpgradeForDeployment(client, namespace, name, sshData, envName)
	} else if rollingUpgradeType == "daemonsets" {
		rollingUpgradeForDaemonSets(client, namespace, name, sshData, envName)
	} else if rollingUpgradeType == "statefulSets" {
		rollingUpgradeForStatefulSets(client, namespace, name, sshData, envName)
	}
}

func rollingUpgradeForDeployment(client kubernetes.Interface, namespace string, name string, sshData string, envName string) error {
	deployments, err := client.ExtensionsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to list deployments %v", err)
	}
	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				updated := updateContainers(containers, name, sshData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the deployment
					_, err := client.ExtensionsV1beta1().Deployments(namespace).Update(&d)
					if err != nil {
						logrus.Fatalf("Update deployment failed %v", err)
					}
					logrus.Infof("Updated Deployment %s", d.Name)
				}
			}
		}
	}
	return nil
}

func rollingUpgradeForDaemonSets(client kubernetes.Interface, namespace string, name string, sshData string, envName string) error {
	daemonSets, err := client.ExtensionsV1beta1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to list daemonSets %v", err)
	}

	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range daemonSets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match daemonSets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				updated := updateContainers(containers, name, sshData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the daemonSet
					_, err := client.ExtensionsV1beta1().DaemonSets(namespace).Update(&d)
					if err != nil {
						logrus.Fatalf("Update daemonSet failed %v", err)
					}
					logrus.Infof("Updated daemonSet %s", d.Name)
				}
			}
		}
	}
	return nil
}

func rollingUpgradeForStatefulSets(client kubernetes.Interface, namespace string, name string, sshData string, envName string) error {
	statefulSets, err := client.AppsV1beta1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to list statefulSets %v", err)
	}
	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range statefulSets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match statefulSets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				updated := updateContainers(containers, name, sshData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the statefulSet
					_, err := client.AppsV1beta1().StatefulSets(namespace).Update(&d)
					if err != nil {
						logrus.Fatalf("Update statefulSet failed %v", err)
					}
					logrus.Infof("Updated statefulSet %s", d.Name)
				}
			}
		}
	}
	return nil
}

func updateContainers(containers []v1.Container, annotationValue string, sshData string, resourceType string) bool {
	updated := false
	envar := "STAKATER_" + helper.ConvertToEnvVarName(annotationValue) + resourceType
	logrus.Infof("Generated environment variable: %s", envar)

	for i := range containers {
		envs := containers[i].Env
		matched := false
		for j := range envs {
			if envs[j].Name == envar {
				matched = true
				logrus.Infof("%s environment variable found", envar)
				if envs[j].Value != sshData {
					logrus.Infof("Updating %s to %s", envar, sshData)
					envs[j].Value = sshData
					updated = true
				}
			}
		}
		// if no existing env var exists lets create one
		if !matched {
			e := v1.EnvVar{
				Name:  envar,
				Value: sshData,
			}
			containers[i].Env = append(containers[i].Env, e)
			updated = true
			logrus.Infof("%s environment variable does not found, creating a new env with value %s", envar, sshData)
		}
	}
	return updated
}
