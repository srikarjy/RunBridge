create table approval_receipts (
    approval_id text primary key references approvals(id) on delete restrict,
    specification_digest text not null check (length(specification_digest) = 64),
    receipt_digest text not null check (length(receipt_digest) = 64),
    created_at timestamptz not null
);

create table execution_receipts (
    execution_id text primary key references executions(id) on delete restrict,
    approval_receipt_digest text not null check (length(approval_receipt_digest) = 64),
    manifest_digest text not null check (length(manifest_digest) = 64),
    receipt_digest text not null check (length(receipt_digest) = 64),
    external_execution_id text not null check (btrim(external_execution_id) <> ''),
    completed_at timestamptz not null,
    created_at timestamptz not null
);
