package shopify

// GetOrderByID is the GraphQL query for retrieving an order by its ID
const getOrderByIDQuery = `
	query orderByIDQuery($id: ID!) {
		order(id: $id) {
			id
			name
      statusPageUrl
			createdAt
			displayFinancialStatus
			displayFulfillmentStatus
			currentTotalPriceSet {
				shopMoney {
					amount
					currencyCode
				}
			}
      transactions(first: 10) {
        amountSet {
          shopMoney {
            amount
            currencyCode
          }
        }
        kind
        status
      }
			lineItems(first: 5) {
				edges {
					node {
						name
						quantity
						sku
					}
				}
			}
			customer {
				displayName
				id
        defaultPhoneNumber {
          phoneNumber
        }
				parentId: metafield(namespace: "customer_fields", key: "parent_id") {
          key
          value
          jsonValue
        }
        directDebit: metafield(namespace: "custom", key: "direct_debito") {
          key
          value
          jsonValue
        }
			}
		}
	}
`

const getOrderByName = `
query orderByName($query: String!, $first: Int!) {
  orders(first: $first, query: $query) {
    nodes {
      id
      name
      statusPageUrl
      createdAt
      displayFinancialStatus
      displayFulfillmentStatus
      currentTotalPriceSet {
        shopMoney {
          amount
          currencyCode
        }
      }
      transactions(first: 10) {
        amountSet {
          shopMoney {
            amount
            currencyCode
          }
        }
        kind
        status
      }
      lineItems(first: 5) {
        edges {
          node {
            name
            quantity
            sku
          }
        }
      }
      customer {
        displayName
        id
        defaultPhoneNumber {
          phoneNumber
        }
        parentId: metafield(namespace: "customer_fields", key: "parent_id") {
          key
          value
          jsonValue
        }
        directDebit: metafield(namespace: "custom", key: "direct_debito") {
          key
          value
          jsonValue
        }
      }
    }
    pageInfo {
      hasNextPage
      startCursor
      hasPreviousPage
      endCursor
    }
  }
}`

const setCustomerMetafield = `mutation UpdateCustomerParentID($id: ID!, $namespace: String!, $key: String!, $value: String!) {
  customerUpdate(input: {
    id: $id
    metafields: [
      {
        namespace: $namespace
        key: $key
        value: $value
      }
    ]
  }) {
    customer {
      id
    }
    userErrors {
      message
      field
    }
  }
}`

const getCustomerMetafield = `
query($id: ID!, $namespace: String!, $key: String!) {
  customer(id: $id) {
    metafield(namespace: $namespace, key: $key) { 
      key
      value
      jsonValue
    }
  }
}`

const markOrderAsPaid = `
mutation orderMarkAsPaid($id: ID!) {
  orderMarkAsPaid(input: { id: $id })
  {
    order {
      id
    }
    userErrors {
      field
      message
    }
  }
}
`
