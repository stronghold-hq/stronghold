# Test: Error handling for invalid inputs
# Verifies CLI rejects bad inputs gracefully.

test_start "Error — wallet replace with no chain arg"
run_cli wallet replace
if [ "$CLI_EXIT" -ne 0 ]; then
    test_pass "wallet replace with no chain arg exits non-zero"
else
    test_fail "wallet replace with no chain arg should exit non-zero"
fi

test_start "Error — wallet replace evm with invalid key"
echo -n "notahexkey" > /tmp/bad-evm-key.txt
chmod 0600 /tmp/bad-evm-key.txt
run_cli wallet replace evm --yes --file /tmp/bad-evm-key.txt
rm -f /tmp/bad-evm-key.txt
if [ "$CLI_EXIT" -ne 0 ]; then
    test_pass "wallet replace evm rejects invalid key (exit $CLI_EXIT)"
else
    test_fail "wallet replace evm should reject invalid key"
fi

test_start "Error — wallet replace solana with invalid key"
echo -n "notavalidkey" > /tmp/bad-solana-key.txt
chmod 0600 /tmp/bad-solana-key.txt
run_cli wallet replace solana --yes --file /tmp/bad-solana-key.txt
rm -f /tmp/bad-solana-key.txt
if [ "$CLI_EXIT" -ne 0 ]; then
    test_pass "wallet replace solana rejects invalid key (exit $CLI_EXIT)"
else
    test_fail "wallet replace solana should reject invalid key"
fi

test_start "Error — config set with invalid key path"
run_cli config set nonexistent.key value
if [ "$CLI_EXIT" -ne 0 ]; then
    test_pass "config set rejects unknown key (exit $CLI_EXIT)"
else
    assert_contains "$CLI_OUTPUT" "unknown" "output indicates unknown key"
fi
