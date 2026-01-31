# File: /kubestellar-integration-toolkit/kubestellar-integration-toolkit/docs/api-reference.md

# API Reference

## Overview

This document provides an overview of the API endpoints available in the KubeStellar Integration Toolkit. The API is designed to facilitate interactions with the `ClusterDeploymentStatus` resources across multiple clusters.

## Base URL

The base URL for the API is:

```
http://<your-api-server>/api/v1alpha1
```

## Endpoints

### Get ClusterDeploymentStatus

```
GET /clusterdeploymentstatuses
```

#### Description

Retrieves a list of `ClusterDeploymentStatus` resources.

#### Response

- **200 OK**: Returns a list of `ClusterDeploymentStatus` objects.

### Create ClusterDeploymentStatus

```
POST /clusterdeploymentstatuses
```

#### Description

Creates a new `ClusterDeploymentStatus` resource.

#### Request Body

- **Content-Type**: application/json
- **Body**: A JSON object representing the `ClusterDeploymentStatus`.

#### Response

- **201 Created**: Returns the created `ClusterDeploymentStatus` object.
- **400 Bad Request**: If the request body is invalid.

### Update ClusterDeploymentStatus

```
PUT /clusterdeploymentstatuses/{name}
```

#### Description

Updates an existing `ClusterDeploymentStatus` resource.

#### Request Body

- **Content-Type**: application/json
- **Body**: A JSON object representing the updated `ClusterDeploymentStatus`.

#### Response

- **200 OK**: Returns the updated `ClusterDeploymentStatus` object.
- **404 Not Found**: If the specified resource does not exist.

### Delete ClusterDeploymentStatus

```
DELETE /clusterdeploymentstatuses/{name}
```

#### Description

Deletes a `ClusterDeploymentStatus` resource.

#### Response

- **204 No Content**: If the deletion was successful.
- **404 Not Found**: If the specified resource does not exist.

## Error Handling

All API responses include a standard error format:

```json
{
  "error": {
    "code": "string",
    "message": "string"
  }
}
```

## Authentication

The API requires authentication via Bearer tokens. Include the token in the Authorization header:

```
Authorization: Bearer <token>
```

## Rate Limiting

The API enforces rate limiting. Exceeding the limit will result in a `429 Too Many Requests` response.

## Versioning

The API is versioned using the URL path. The current version is `v1alpha1`. Future versions will follow the same pattern.

## Contact

For support, please contact the maintainers of the KubeStellar Integration Toolkit.