// +kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=globalMeshs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=globalMeshs/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshs/status,verbs=get;update;patch

package v1alpha1
