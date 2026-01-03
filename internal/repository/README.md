# Repository Package

The repository package implements the data access layer using the repository pattern.

## Overview

This package provides:
- Clean separation between business logic and data access
- Database abstraction for testability
- CRUD operations for domain entities

## Repositories

### UserRepository

```go
type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    FindByID(ctx context.Context, id string) (*models.User, error)
    FindByEmail(ctx context.Context, email string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id string) error
}
```

### SessionRepository

```go
type SessionRepository interface {
    Create(ctx context.Context, session *models.Session) error
    FindByID(ctx context.Context, id string) (*models.Session, error)
    FindByUserID(ctx context.Context, userID string) ([]*models.Session, error)
    Delete(ctx context.Context, id string) error
}
```

### ProviderRepository

```go
type ProviderRepository interface {
    GetAll(ctx context.Context) ([]*models.Provider, error)
    GetByID(ctx context.Context, id string) (*models.Provider, error)
    Create(ctx context.Context, provider *models.Provider) error
    Update(ctx context.Context, provider *models.Provider) error
    Delete(ctx context.Context, id string) error
}
```

## Usage

### Creating a Repository

```go
db := database.NewConnection(cfg)
userRepo := repository.NewUserRepository(db)
```

### Using a Repository

```go
// Create
err := userRepo.Create(ctx, &models.User{
    Email: "user@example.com",
    Name:  "John Doe",
})

// Find
user, err := userRepo.FindByEmail(ctx, "user@example.com")

// Update
user.Name = "Jane Doe"
err = userRepo.Update(ctx, user)

// Delete
err = userRepo.Delete(ctx, user.ID)
```

## Testing

Repositories can be mocked for testing:

```go
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*models.User), args.Error(1)
}
```

## Transaction Support

For operations requiring transactions:

```go
err := repo.WithTransaction(ctx, func(tx *sql.Tx) error {
    // Multiple repository operations within transaction
    if err := userRepo.Create(ctx, user); err != nil {
        return err
    }
    if err := sessionRepo.Create(ctx, session); err != nil {
        return err
    }
    return nil
})
```
