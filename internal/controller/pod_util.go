package controller

import (
	batchv1 "k8s.io/api/batch/v1"
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

func JobIsCompleted(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
