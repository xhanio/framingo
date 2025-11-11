# Types Package

This package contains different type definitions used throughout the application, organized by their purpose.

## Directory Structure

### `api/`
Contains request and response types used for API endpoints.

**Purpose**: Define the structure of data sent to and from API endpoints.

**Characteristics**:
- Include validation tags (`validate:"required"`, etc.)
- Support multiple binding formats (`json`, `form`, `query`)
- Used for request parsing and validation

**Example**:
```go
type CreateHelloworldMessage struct {
    Message string `json:"message" form:"message" query:"message" validate:"required"`
}
```

### `entity/`
Contains business domain entities representing the core data models.

**Purpose**: Define the pure business logic representation of data without infrastructure concerns.

**Characteristics**:
- Include only JSON tags for serialization
- No database-specific tags (GORM, SQL, etc.)
- Represent the domain model independent of storage mechanism
- Used in business logic and API responses

**Example**:
```go
type Helloworld struct {
    ID        int64     `json:"id"`
    Message   string    `json:"message"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### `orm/`
Contains ORM (Object-Relational Mapping) models for database operations.

**Purpose**: Define the database schema and mapping for persistence layer.

**Characteristics**:
- Include GORM-specific tags (`gorm:"primaryKey"`, `gorm:"type:text"`, etc.)
- Implement `TableName()` method to specify database table names
- Used exclusively for database operations (CRUD)
- Should be converted to/from entities in the service layer

**Example**:
```go
type Helloworld struct {
    ID        int64     `gorm:"primaryKey"`
    Message   string    `gorm:"type:text;not null"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Helloworld) TableName() string {
    return "helloworld_messages"
}
```

## Design Philosophy

The separation of types follows the principle of **Separation of Concerns**:

1. **API types** handle external communication and validation
2. **Entity types** represent pure business logic
3. **ORM types** handle database persistence

This separation provides several benefits:

- **Flexibility**: Change database implementation without affecting business logic
- **Testability**: Business logic can be tested without database concerns
- **Maintainability**: Clear boundaries between layers
- **Independence**: API contracts are independent of storage implementation

## Usage Pattern

```go
// In service layer
func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error) {
    // Create ORM model for database operation
    ormModel := &orm.Helloworld{
        Message: message,
    }

    // Save to database using ORM
    if err := m.db.FromContext(ctx).Create(ormModel).Error; err != nil {
        return nil, errors.Wrap(err)
    }

    // Convert ORM to entity for return
    result := &entity.Helloworld{
        ID:        ormModel.ID,
        Message:   ormModel.Message,
        CreatedAt: ormModel.CreatedAt,
        UpdatedAt: ormModel.UpdatedAt,
    }

    return result, nil
}
```

```go
// In handler layer
func (r *router) Example(c echo.Context) error {
    // Parse API request
    var req api.CreateHelloworldMessage
    if err := c.Bind(&req); err != nil {
        return errors.BadRequest.Newf("invalid request: %v", err)
    }

    // Validate
    if err := c.Validate(&req); err != nil {
        return errors.Wrap(err)
    }

    // Call service (returns entity)
    body, err := r.em.HelloWorld(c.Request().Context(), req.Message)
    if err != nil {
        return errors.Wrap(err)
    }

    // Return entity as JSON response
    return c.JSON(http.StatusOK, body)
}
```
