package converters

import (
	"github.com/haproxytech/client-native/v3/models"
	corev1alpha1 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha1"
)

type (
	Template  corev1alpha1.BackendSpec
	NewCPUMap models.H1CaseAdjust
)
