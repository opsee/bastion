package schema

import (
	"math/rand"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/graphql-go/graphql"
)

func TestUnmarshalCheck(t *testing.T) {
	checkString := `{"id":"www.reddit.com","interval":0,"target":{"name":"www.reddit.com","type":"url","id":"www.reddit.com","address":"www.reddit.com"},"name":"www.reddit.com","http_check":{"name":"","path":"/r/pepe","protocol":"https","port":443,"verb":"GET","body":""}}`
	check := &Check{}
	err := jsonpb.UnmarshalString(checkString, check)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckschema(t *testing.T) {
	popr := rand.New(rand.NewSource(time.Now().UnixNano()))
	check := NewPopulatedCheck(popr, false)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"check": &graphql.Field{
					Type: GraphQLCheckType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return check, nil
					},
				},
			},
		}),
	})

	if err != nil {
		t.Fatal(err)
	}

	queryResponse := graphql.Do(graphql.Params{Schema: schema, RequestString: `query yeOldeQuery {
		check {
			id
			interval
			target {
				name
				type
				id
				address
			}
			last_run
			spec {
				... on schemaHttpCheck {
					name
					path
					protocol
					port
					verb
					headers {
						name
						values
					}
					body
				}
				... on schemaCloudWatchCheck {
					metrics {
						namespace
						name
					}

				}
			}
			name
			assertions {
				key
				value
				relationship
				operand
			}
			results {
				check_id
				customer_id
				timestamp
				passing
				responses {
					target {
						name
						type
						id
						address
					}
					response
					error
					passing
					reply {
						... on schemaHttpResponse {
							code
							body
							headers {
								name
								values
							}
							metrics {
								name
								value
								tags {
									name
									value
								}
								timestamp
								unit
								statistic
							}
							host
						}
					}
				}
				target {
					name
					type
					id
					address
				}
				check_name
				version
			}
		}
	}`})

	if queryResponse.HasErrors() {
		t.Fatalf("graphql query errors: %#v\n", queryResponse.Errors)
	}
}
