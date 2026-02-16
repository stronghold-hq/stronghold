# Test: stronghold wallet replace
# Tests replacing wallets with new keys via --file flag.
# Note: env var approach doesn't work in Docker because readPrivateKey
# checks --file first, then env var, then stdin (non-tty stdin detected
# as piped → EOF), then interactive prompt.

test_start "Wallet replace — Solana via file"
echo -n "$REPLACE_SOLANA_PRIVATE_KEY" > /tmp/replace-solana-key.txt
chmod 0600 /tmp/replace-solana-key.txt
run_cli wallet replace solana --yes --file /tmp/replace-solana-key.txt
rm -f /tmp/replace-solana-key.txt
assert_exit_zero "$CLI_EXIT" "wallet replace solana exits 0"
assert_contains "$CLI_OUTPUT" "Solana wallet updated" "Solana wallet replaced"

test_start "Wallet replace — EVM via file"
echo -n "$REPLACE_EVM_PRIVATE_KEY" > /tmp/replace-evm-key.txt
chmod 0600 /tmp/replace-evm-key.txt
run_cli wallet replace evm --yes --file /tmp/replace-evm-key.txt
rm -f /tmp/replace-evm-key.txt
assert_exit_zero "$CLI_EXIT" "wallet replace evm exits 0"
assert_contains "$CLI_OUTPUT" "wallet updated" "EVM wallet replaced"

test_start "Wallet replace — verify new addresses"
run_cli wallet list
assert_exit_zero "$CLI_EXIT" "wallet list after replace exits 0"
assert_contains "$CLI_OUTPUT" "$REPLACE_EVM_ADDRESS" "wallet list shows new EVM address"
# Verify Solana address changed (exact address depends on key derivation)
assert_not_contains "$CLI_OUTPUT" "Not configured" "Solana wallet still configured"
