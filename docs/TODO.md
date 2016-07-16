# TODO

The following document outlines the list of behaviours that need to be added / changed / fixed before version 1.0.0

## Bugs

- [FE] Intermittent issue with storing information against a link causes all links to change to the same value
- [FE] Refreshing page on child bucket does not remember location

## Must have
- Flow executor
- Syphon executor
- [FE] Help text for off-canvas sidebar
- [FE] delete functionality for buckets
- [FE] Create functionality for key/values
- [FE] Pipeline canvas Zoom/reset functionality
- [BE] Logging
- [ALL] Create test suite for all existing paths
- Authentication
- Session management
- Environment handling
- As a component is executing, report its status / log output in the front end

## Should have
- [FE] Pipeline container grouping.
  JointJS offers automatic parenting of objects placed inside containers.
  - Create: Container structure for each offered deployment type
    - Kubernetes: Namespace, Deployment, StatefulSet, DaemonSet

- [FE/DB] Automatic base64 decoding/encoding against the inline editor for JSON structured values
- Expand tool set to cover drag-drop building of cloud components using terraform for infrastructure and ansible for
  configuration.
- Dashboard should have cluster status
- Credential integration using hashicorp vault

## Could have
- Git integration for script source
- Zip/tar upload integration for script source
- Cloud native infrastructure (Azure, AWS, GCP)
- Terraform/Ansible integration for cloud components

## Would have
- Pipeline components should be able to be instructed to use existing deployments
- Nested pipelines
- Component grouping (e.g. single container running multiple components)

