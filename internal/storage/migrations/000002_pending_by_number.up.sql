DROP INDEX orders_pending_idx;
CREATE INDEX orders_pending_idx ON orders (number) WHERE status IN ('NEW', 'PROCESSING');
