package shopify

// GetOrderByID is the GraphQL query for retrieving an order by its ID
const getOrderByIDQuery = `
	query orderByIDQuery($id: ID!) {
		order(id: $id) {
			id
			name
      tags
      app {
        id
      }
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
			lineItems(first: 10) {
				edges {
					node {
						name
						quantity
						sku
            totalDiscountSet {
              shopMoney{
                amount
              }
            }
					}
				}
			}
			customer {
				displayName
				id
				email
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
        directDebitAccount: metafield(namespace: "custom", key: "direct_debit_account") {
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
      tags
      app {
        id
      }
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
            totalDiscountSet {
              shopMoney{
                amount
              }
            }
          }
        }
      }
      customer {
        displayName
        id
        email
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
        directDebitAccount: metafield(namespace: "custom", key: "direct_debit_account") {
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

const addOrderTags = ` mutation OrderAddTags($id: ID!, $tags: [String!]!) {
  tagsAdd(id: $id, tags: $tags) {
    userErrors {
      field
      message
    }
  }
}`

const beginOrderEdit = `mutation BeginOrderEdit($id: ID!) {
  orderEditBegin(id: $id) {
    calculatedOrder {
      id
      lineItems(first: 10) {
        nodes {
          id
        }
      }
    }
    userErrors {
      field
      message
    }
  }
} `

const addThirtyPercentDiscountToLineItem = `mutation AddThirtyPercentDiscountToLineItem(
  $calculatedOrderId: ID!
  $calculatedLineItemId: ID!
  $description: String!
  $percentValue: Float
) {
  orderEditAddLineItemDiscount(
    id: $calculatedOrderId
    lineItemId: $calculatedLineItemId
    discount: { description: $description, percentValue: $percentValue }
  ) {
    calculatedOrder {
      id
    }
    calculatedLineItem {
      id
    }
    userErrors {
      field
      message
    }
  }
}`

const commitOrderEdit = `mutation CommitOrderEdit($calculatedOrderId: ID!) {
  orderEditCommit(id: $calculatedOrderId, notifyCustomer: false) {
    order {
      id
    }
    userErrors {
      field
      message
    }
  }
}`

const getCustomerByID = `
query($id: ID!) {
  customer(id: $id) {
    id
    displayName
    email
    directDebitAccount: metafield(namespace: "custom", key: "direct_debit_account") {
      key
      value
      jsonValue
    }
  }
}`

const getDraftOrderByID = `
query draftOrderByID($id: ID!) {
  draftOrder(id: $id) {
    id
    name
    tags
    totalPriceSet {
      shopMoney {
        amount
        currencyCode
      }
    }
    customer {
      id
      displayName
      email
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
      directDebitAccount: metafield(namespace: "custom", key: "direct_debit_account") {
        key
        value
        jsonValue
      }
    }
  }
}`

const draftOrderComplete = `
mutation draftOrderComplete($id: ID!, $paymentPending: Boolean!) {
  draftOrderComplete(id: $id, paymentPending: $paymentPending) {
    draftOrder {
      id
      order {
        id
        name
        legacyResourceId
        statusPageUrl
        displayFinancialStatus
      }
    }
    userErrors {
      field
      message
    }
  }
}`

const deleteMetafield = `mutation MetafieldDelete($id: ID!, $namespace: String!, $key: String!) {
  metafieldsDelete(metafields: [
    {
      ownerId: $id
      namespace: $namespace
      key:  $key
    }
  ]) {
    deletedMetafields {
      ownerId
      namespace
      key
    }
    userErrors {
      field
      message
    }
  }
}`
