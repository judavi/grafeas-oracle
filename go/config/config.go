package config

// DynamoDbConfig is the configuration for an AWS DynamoDB store.
type OracleConfig struct {
	Host          string `mapstructure:"host"`
	DbName        string `mapstructure:"dbname"`
	User          string `mapstructure:"user"`
	Password      string `mapstructure:"password"`
	PaginationKey string `mapstructure:"paginationkey"`
}
