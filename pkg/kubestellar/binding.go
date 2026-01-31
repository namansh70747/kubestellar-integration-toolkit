package kubestellar

// This file previously contained binding-related methods.
// All BindingPolicy methods are now consolidated in client.go to avoid duplication:
// - CreateBindingPolicy
// - GetBindingPolicy
// - UpdateBindingPolicy
// - DeleteBindingPolicy
// - GetBindingPolicyStatus
// - AddClusterSelector
// - ListBindingPolicies
//
// The BindingPolicy, ClusterSelector, SelectorRequirement, and DownSyncRule types
// are also defined in client.go.
