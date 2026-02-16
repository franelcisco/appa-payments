CREATE TABLE IF NOT EXISTS r4_appa_debits_direct (
    id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
    sender_phone varchar(20) NOT NULL,
    issuing_bank varchar(100) NOT NULL,
    amount numeric(10,2) NOT NULL,
    reference varchar(100) NOT NULL,
    dni varchar(50) NOT NULL,
    code varchar(10),
    success boolean DEFAULT FALSE,
    order_id varchar(100),
    order_name varchar(100),
    date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);


CREATE UNIQUE INDEX idx_r4_appa_debits_direct_id ON r4_appa_debits_direct(id);
CREATE INDEX idx_r4_appa_debits_direct_sender_phone ON r4_appa_debits_direct(sender_phone);
CREATE INDEX idx_r4_appa_debits_direct_reference ON r4_appa_debits_direct(reference);
CREATE INDEX idx_r4_appa_debits_direct_dni ON r4_appa_debits_direct(dni);
CREATE INDEX idx_r4_appa_debits_direct_order_id ON r4_appa_debits_direct(order_id);
CREATE INDEX idx_r4_appa_debits_direct_date ON r4_appa_debits_direct(date);