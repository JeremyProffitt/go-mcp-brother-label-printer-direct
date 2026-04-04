package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBStore struct {
	client    *dynamodb.Client
	tableName string
}

func NewDynamoDBStore(client *dynamodb.Client, tableName string) *DynamoDBStore {
	return &DynamoDBStore{
		client:    client,
		tableName: tableName,
	}
}

type record struct {
	PK  string `dynamodbav:"PK"`
	SK  string `dynamodbav:"SK"`
	TTL int64  `dynamodbav:"ttl,omitempty"`
}

type clientRecord struct {
	record
	ClientID     string   `dynamodbav:"client_id"`
	ClientName   string   `dynamodbav:"client_name"`
	RedirectURIs []string `dynamodbav:"redirect_uris"`
	GrantTypes   []string `dynamodbav:"grant_types"`
	Scope        string   `dynamodbav:"scope"`
	IssuedAt     int64    `dynamodbav:"issued_at"`
}

type authCodeRecord struct {
	record
	Code          string `dynamodbav:"code"`
	ClientID      string `dynamodbav:"client_id"`
	RedirectURI   string `dynamodbav:"redirect_uri"`
	CodeChallenge string `dynamodbav:"code_challenge"`
	Scope         string `dynamodbav:"scope"`
	UserID        string `dynamodbav:"user_id"`
	Used          bool   `dynamodbav:"used"`
	ExpiresAt     int64  `dynamodbav:"expires_at"`
}

type refreshTokenRecord struct {
	record
	Token     string `dynamodbav:"token"`
	ClientID  string `dynamodbav:"client_id"`
	UserID    string `dynamodbav:"user_id"`
	Scope     string `dynamodbav:"scope"`
	Revoked   bool   `dynamodbav:"revoked"`
	ExpiresAt int64  `dynamodbav:"expires_at"`
}

func (s *DynamoDBStore) SaveClient(ctx context.Context, client *OAuthClient) error {
	rec := clientRecord{
		record:       record{PK: "CLIENT#" + client.ClientID, SK: "CLIENT"},
		ClientID:     client.ClientID,
		ClientName:   client.ClientName,
		RedirectURIs: client.RedirectURIs,
		GrantTypes:   client.GrantTypes,
		Scope:        client.Scope,
		IssuedAt:     client.IssuedAt,
	}
	item, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return fmt.Errorf("marshal client: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	return err
}

func (s *DynamoDBStore) GetClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "CLIENT#" + clientID},
			"SK": &types.AttributeValueMemberS{Value: "CLIENT"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}
	var rec clientRecord
	if err := attributevalue.UnmarshalMap(result.Item, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal client: %w", err)
	}
	return &OAuthClient{
		ClientID:     rec.ClientID,
		ClientName:   rec.ClientName,
		RedirectURIs: rec.RedirectURIs,
		GrantTypes:   rec.GrantTypes,
		Scope:        rec.Scope,
		IssuedAt:     rec.IssuedAt,
	}, nil
}

func (s *DynamoDBStore) SaveAuthCode(ctx context.Context, code *AuthCode) error {
	rec := authCodeRecord{
		record:        record{PK: "AUTHCODE#" + code.Code, SK: "AUTHCODE", TTL: code.ExpiresAt.Unix()},
		Code:          code.Code,
		ClientID:      code.ClientID,
		RedirectURI:   code.RedirectURI,
		CodeChallenge: code.CodeChallenge,
		Scope:         code.Scope,
		UserID:        code.UserID,
		Used:          false,
		ExpiresAt:     code.ExpiresAt.Unix(),
	}
	item, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return fmt.Errorf("marshal auth code: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	return err
}

func (s *DynamoDBStore) GetAuthCode(ctx context.Context, code string) (*AuthCode, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "AUTHCODE#" + code},
			"SK": &types.AttributeValueMemberS{Value: "AUTHCODE"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get auth code: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}
	var rec authCodeRecord
	if err := attributevalue.UnmarshalMap(result.Item, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal auth code: %w", err)
	}
	return &AuthCode{
		Code:          rec.Code,
		ClientID:      rec.ClientID,
		RedirectURI:   rec.RedirectURI,
		CodeChallenge: rec.CodeChallenge,
		Scope:         rec.Scope,
		UserID:        rec.UserID,
		Used:          rec.Used,
		ExpiresAt:     time.Unix(rec.ExpiresAt, 0),
	}, nil
}

func (s *DynamoDBStore) MarkAuthCodeUsed(ctx context.Context, code string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "AUTHCODE#" + code},
			"SK": &types.AttributeValueMemberS{Value: "AUTHCODE"},
		},
		UpdateExpression: aws.String("SET used = :true"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true": &types.AttributeValueMemberBOOL{Value: true},
		},
	})
	return err
}

func (s *DynamoDBStore) SaveRefreshToken(ctx context.Context, token *RefreshToken) error {
	rec := refreshTokenRecord{
		record:    record{PK: "REFRESH#" + token.Token, SK: "REFRESH", TTL: token.ExpiresAt.Unix()},
		Token:     token.Token,
		ClientID:  token.ClientID,
		UserID:    token.UserID,
		Scope:     token.Scope,
		Revoked:   false,
		ExpiresAt: token.ExpiresAt.Unix(),
	}
	item, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return fmt.Errorf("marshal refresh token: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	return err
}

func (s *DynamoDBStore) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "REFRESH#" + token},
			"SK": &types.AttributeValueMemberS{Value: "REFRESH"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}
	var rec refreshTokenRecord
	if err := attributevalue.UnmarshalMap(result.Item, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token: %w", err)
	}
	return &RefreshToken{
		Token:     rec.Token,
		ClientID:  rec.ClientID,
		UserID:    rec.UserID,
		Scope:     rec.Scope,
		Revoked:   rec.Revoked,
		ExpiresAt: time.Unix(rec.ExpiresAt, 0),
	}, nil
}

func (s *DynamoDBStore) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "REFRESH#" + token},
			"SK": &types.AttributeValueMemberS{Value: "REFRESH"},
		},
		UpdateExpression: aws.String("SET revoked = :true"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true": &types.AttributeValueMemberBOOL{Value: true},
		},
	})
	return err
}

// ValidRedirectURI checks if the given URI is in the client's registered redirect URIs.
func ValidRedirectURI(client *OAuthClient, uri string) bool {
	for _, registered := range client.RedirectURIs {
		if registered == uri {
			return true
		}
		if strings.HasPrefix(registered, "http://localhost") && strings.HasPrefix(uri, "http://localhost") {
			return true
		}
	}
	return false
}
