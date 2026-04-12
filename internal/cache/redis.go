package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spacebxr/strelp/internal/models"
)

type Cache struct {
	client *redis.Client
}

func NewCache(addr string, password string, db int) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return &Cache{client: client}, nil
}

func (c *Cache) SetPresence(ctx context.Context, userID string, presence *models.Presence) error {
	data, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("strelp:presence:%s", userID)
	if err := c.client.Set(ctx, key, data, 0).Err(); err != nil {
		return err
	}

	return c.client.Publish(ctx, fmt.Sprintf("strelp:updates:%s", userID), data).Err()
}

func (c *Cache) GetPresence(ctx context.Context, userID string) (*models.Presence, error) {
	key := fmt.Sprintf("strelp:presence:%s", userID)
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var presence models.Presence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (c *Cache) DeletePresence(ctx context.Context, userID string) error {
	key := fmt.Sprintf("strelp:presence:%s", userID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	return c.client.Publish(ctx, fmt.Sprintf("strelp:updates:%s", userID), "DELETED").Err()
}

func (c *Cache) Subscribe(ctx context.Context, userID string) *redis.PubSub {
	return c.client.Subscribe(ctx, fmt.Sprintf("strelp:updates:%s", userID))
}
