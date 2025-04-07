// TODO make this file a patch to k8s.io/util or somewhere else or publish as individual package
package conditions

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func IsTrue(conditionType string, conds []v1.Condition) bool {
	for _, cond := range conds {
		if cond.Type != conditionType {
			continue
		}
		return cond.Status == v1.ConditionTrue
	}
	return false
}

func IsFalse(conditionType string, conds []v1.Condition) bool {
	return !IsTrue(conditionType, conds)
}
