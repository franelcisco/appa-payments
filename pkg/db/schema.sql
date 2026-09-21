CREATE TABLE IF NOT EXISTS r4_appa_debits_direct_account (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    store_client_id varchar(100) NOT NULL,
    account varchar(100) NOT NULL,
    amount numeric(10,2) NOT NULL,
    reference varchar(100) NOT NULL,
    dni varchar(50) NOT NULL,
    code varchar(10),
    operation_id varchar(100),
    success boolean DEFAULT FALSE,
    order_id varchar(100),
    order_name varchar(100),
    draft_id varchar(100),
    is_recurring boolean DEFAULT FALSE,
    cart_id varchar(100),
    date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_r4_appa_debits_direct_account_id ON r4_appa_debits_direct_account(id);
CREATE INDEX idx_r4_appa_debits_direct_account_store_client_id ON r4_appa_debits_direct_account(store_client_id);
CREATE INDEX idx_r4_appa_debits_direct_account_account ON r4_appa_debits_direct_account(account);
CREATE INDEX idx_r4_appa_debits_direct_account_reference ON r4_appa_debits_direct_account(reference);
CREATE INDEX idx_r4_appa_debits_direct_account_dni ON r4_appa_debits_direct_account(dni);
CREATE INDEX idx_r4_appa_debits_direct_account_operation_id ON r4_appa_debits_direct_account(operation_id);

CREATE TABLE IF NOT EXISTS r4_appa_debits_direct (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    sender_phone varchar(20) NOT NULL,
    issuing_bank varchar(100) NOT NULL,
    amount numeric(10,2) NOT NULL,
    reference varchar(100) NOT NULL,
    dni varchar(50) NOT NULL,
    code varchar(10),
    operation_id varchar(100),
    success boolean DEFAULT FALSE,
    order_id varchar(100),
    order_name varchar(100),
    order_type varchar(20),
    cart_id varchar(100),
    date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);


CREATE UNIQUE INDEX idx_r4_appa_debits_direct_id ON r4_appa_debits_direct(id);
CREATE INDEX idx_r4_appa_debits_direct_sender_phone ON r4_appa_debits_direct(sender_phone);
CREATE INDEX idx_r4_appa_debits_direct_reference ON r4_appa_debits_direct(reference);
CREATE INDEX idx_r4_appa_debits_direct_dni ON r4_appa_debits_direct(dni);
CREATE INDEX idx_r4_appa_debits_direct_order_id ON r4_appa_debits_direct(order_id);
CREATE INDEX idx_r4_appa_debits_direct_operation_id ON r4_appa_debits_direct(operation_id);
CREATE INDEX idx_r4_appa_debits_direct_date ON r4_appa_debits_direct(date);

CREATE TABLE IF NOT EXISTS r4_appa_recurrent_pending_payments (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    order_id varchar(100) NOT NULL,
    order_name varchar(100),
    attempts int4 DEFAULT 1,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_r4_appa_recurrent_pending_payments_order_id ON r4_appa_recurrent_pending_payments(order_id);

CREATE TABLE IF NOT EXISTS r4_appa_mobile_payments_reversals (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    reference varchar(100),
    order_name varchar(100),
    order_amount numeric(10,2) NOT NULL,
    reversal_amount numeric(10,2) NOT NULL,
    reason varchar(20),
    success boolean DEFAULT FALSE,
    error_detail text,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_r4_appa_mobile_payments_reversals_id ON r4_appa_mobile_payments_reversals(id);
-- Vueltos sent OUT on behalf of a caller (APPA claim payouts), one row per
-- payout id. The UNIQUE index on payout_id is load-bearing: it is the
-- idempotency key MBvuelto doesn't have. POST /payouts/vuelto reserves the id
-- here BEFORE calling R4 and refuses to pay if this table or index is missing.
-- numeric(14,2): clinic statements in Bs don't fit the numeric(10,2) used above.
CREATE TABLE IF NOT EXISTS r4_appa_payouts (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    payout_id varchar(100) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    bank varchar(10) NOT NULL,
    phone varchar(20) NOT NULL,
    dni varchar(20) NOT NULL,
    amount numeric(14,2) NOT NULL,
    concept varchar(100),
    reference varchar(100),
    detail text,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_r4_appa_payouts_payout_id ON r4_appa_payouts(payout_id);
