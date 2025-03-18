package controller

import (
	corev1 "k8s.io/api/core/v1"
)

func PodIsReadyOrFinished(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodSucceeded {
		return true
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}
		if cond.Status != corev1.ConditionTrue {
			continue
		}
		return true
	}
	return false
}
