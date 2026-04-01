// payment_system.go
// Extremely insecure legacy payment system in Go
// Educational bad code example - full of SQL injection, plain text secrets, massive duplication

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const (
	DB_HOST = "localhost"
	DB_PORT = "5432"
	DB_NAME = "payment_legacy_db"
	DB_USER = "postgres"
	DB_PASS = "SuperSecret123!"
	SITE_SECRET = "myglobalsecret123"
)

var GLOBAL_DB *sql.DB

func getDB() *sql.DB {
	if GLOBAL_DB == nil {
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME)
		var err error
		GLOBAL_DB, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Fatal("CRITICAL DATABASE FAILURE: ", err)
		}
		_, err = GLOBAL_DB.Exec("SET client_encoding = 'UTF8';")
		if err != nil {
			log.Println("Warning: failed to set encoding")
		}
	}
	return GLOBAL_DB
}

func register_customer(username, email, password, full_name, phone, country, city, address string) string {
	db := getDB()
	id := "cust_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	sqlStr := `INSERT INTO customers (
        id, username, email, password, full_name, phone, country, city, address_line_1,
        created_at, updated_at, register_ip, user_agent, is_admin, role_name
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()::text, NOW()::text, '127.0.0.1', 'GO-LEGACY', 'false', 'customer'
    ) RETURNING id;`

	var newID string
	err := db.QueryRow(sqlStr, id, username, email, password, full_name, phone, country, city, address).Scan(&newID)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return ""
	}
	fmt.Println("Customer registered ID:", newID)
	return newID
}

func login_customer(username, password string) string {
	db := getDB()
	sqlStr := `SELECT * FROM customers WHERE username = '` + username + `' AND password = '` + password + `' LIMIT 1;`
	rows, err := db.Query(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return ""
	}
	defer rows.Close()

	if rows.Next() {
		var user struct {
			ID string
		}
		rows.Scan(&user.ID /* other fields ignored for simplicity */)
		sessionToken := fmt.Sprintf("%x", []byte(user.ID+fmt.Sprintf("%d", time.Now().Unix())+SITE_SECRET))
		update := `UPDATE customers SET session_token = '` + sessionToken + `', last_login_ip = '127.0.0.1', failed_login_count = '0', updated_at = NOW()::text WHERE id = '` + user.ID + `';`
		db.Exec(update)
		fmt.Println("LOGIN SUCCESS Session:", sessionToken)
		return sessionToken
	}

	failSQL := `UPDATE customers SET failed_login_count = (failed_login_count::int + 1)::text WHERE username = '` + username + `';`
	db.Exec(failSQL)
	fmt.Println("LOGIN FAILED")
	return ""
}

func get_customer(customer_id string) map[string]interface{} {
	db := getDB()
	sqlStr := `SELECT * FROM customers WHERE id = '` + customer_id + `' LIMIT 1;`
	rows, err := db.Query(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var row map[string]interface{}
		rows.Scan(&row)
		return row
	}
	return nil
}

func update_customer_profile(customer_id, new_email, new_phone, new_address string) {
	db := getDB()
	sqlStr := `UPDATE customers SET email = '` + new_email + `', phone = '` + new_phone + `', address_line_1 = '` + new_address + `', updated_at = NOW()::text WHERE id = '` + customer_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Customer profile updated")
}

func reset_password(email, new_password string) {
	db := getDB()
	sqlStr := `UPDATE customers SET password = '` + new_password + `', reset_token = 'reset_' || md5(NOW()::text), reset_token_expires_at = (NOW() + INTERVAL '1 day')::text WHERE email = '` + email + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Password reset token generated for", email)
}

func verify_email(token string) {
	db := getDB()
	sqlStr := `UPDATE customers SET email_verification_token = NULL WHERE email_verification_token = '` + token + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Email verified with token", token)
}

func add_payment_method(customer_id, typ, card_number, expiry_month, expiry_year, cvv, holder_name, iban string) string {
	db := getDB()
	id := "pm_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	sqlStr := `INSERT INTO payment_methods (
        id, customer_id, type, provider, card_number, card_expiry_month, card_expiry_year, 
        card_cvv, card_holder_name, iban, active_flag, created_at, updated_at
    ) VALUES (
        '` + id + `', '` + customer_id + `', '` + typ + `', 'legacy_bank_gateway', '` + card_number + `', '` + expiry_month + `', '` + expiry_year + `', '` + cvv + `', '` + holder_name + `', '` + iban + `', 'true', NOW()::text, NOW()::text
    ) RETURNING id;`
	var newID string
	err := db.QueryRow(sqlStr).Scan(&newID)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return ""
	}
	fmt.Println("Payment method added ID:", newID)
	return newID
}

func list_payment_methods(customer_id string) []map[string]interface{} {
	db := getDB()
	sqlStr := `SELECT * FROM payment_methods WHERE customer_id = '` + customer_id + `' AND deleted_at IS NULL;`
	rows, err := db.Query(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return nil
	}
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var row map[string]interface{}
		rows.Scan(&row)
		results = append(results, row)
	}
	return results
}

func delete_payment_method(pm_id string) {
	db := getDB()
	sqlStr := `UPDATE payment_methods SET deleted_at = NOW()::text WHERE id = '` + pm_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Payment method deleted")
}

func process_payment(customer_id, payment_method_id, amount, currency, external_order_id, ip string) string {
	db := getDB()
	id := "pay_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	if ip == "" {
		ip = "127.0.0.1"
	}
	if external_order_id == "" {
		external_order_id = "ord_" + fmt.Sprintf("%d", time.Now().Unix())
	}
	rawPayload := `{"card_number":"****4242","provider_secret":"sk_live_9876543210abcdef","cvv_used":"123","3ds_password":"customer123"}`

	sqlStr := `INSERT INTO payments (
        id, customer_id, payment_method_id, external_order_id, amount, currency, status,
        provider_ref, ip_address, raw_provider_payload, created_at, paid_at, captured_flag
    ) VALUES (
        '` + id + `', '` + customer_id + `', '` + payment_method_id + `', '` + external_order_id + `', '` + amount + `', '` + currency + `', 'captured',
        'prov_` + fmt.Sprintf("%d", time.Now().Unix()) + `', '` + ip + `', '` + rawPayload + `', NOW()::text, NOW()::text, 'true'
    ) RETURNING id;`

	var payID string
	err := db.QueryRow(sqlStr).Scan(&payID)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return ""
	}

	update := `UPDATE customers SET total_paid = (COALESCE(total_paid::numeric, 0) + ` + amount + `)::text WHERE id = '` + customer_id + `';`
	db.Exec(update)

	logSQL := `INSERT INTO payment_logs (id, payment_id, customer_id, log_level, message, payload, created_at, actor_email, source)
               VALUES ('log_' || nextval('payment_logs_id_seq'::regclass), '` + payID + `', '` + customer_id + `', 'INFO', 'Payment captured successfully', '` + rawPayload + `', NOW()::text, 'system@legacy.com', 'legacy_core');`
	db.Exec(logSQL)

	fmt.Println("PAYMENT PROCESSED ID:", payID, "Amount:", amount, currency)
	return payID
}

func list_payments(customer_id string) []map[string]interface{} {
	db := getDB()
	sqlStr := `SELECT * FROM payments WHERE customer_id = '` + customer_id + `' ORDER BY created_at DESC;`
	rows, err := db.Query(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return nil
	}
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var row map[string]interface{}
		rows.Scan(&row)
		results = append(results, row)
	}
	return results
}

func get_payment_details(payment_id string) map[string]interface{} {
	db := getDB()
	sqlStr := `SELECT * FROM payments WHERE id = '` + payment_id + `' LIMIT 1;`
	rows, err := db.Query(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return nil
	}
	defer rows.Close()
	if rows.Next() {
		var row map[string]interface{}
		rows.Scan(&row)
		return row
	}
	return nil
}

func create_refund(payment_id, amount, reason string) {
	db := getDB()
	id := "ref_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	sqlStr := `INSERT INTO refunds (id, payment_id, amount, currency, status, reason, created_at)
               VALUES ('` + id + `', '` + payment_id + `', '` + amount + `', 'EUR', 'pending', '` + reason + `', NOW()::text);`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Refund created for payment", payment_id)
}

func process_refund(refund_id string) {
	db := getDB()
	sqlStr := `UPDATE refunds SET status = 'processed', processed_at = NOW()::text WHERE id = '` + refund_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Refund processed ID:", refund_id)
}

func simulate_chargeback(payment_id, amount, reason string) {
	db := getDB()
	id := "cb_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	sqlStr := `INSERT INTO chargebacks (id, payment_id, amount, currency, reason, status, created_at, deadline_at)
               VALUES ('` + id + `', '` + payment_id + `', '` + amount + `', 'EUR', '` + reason + `', 'open', NOW()::text, (NOW() + INTERVAL '7 days')::text);`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Chargeback created for payment", payment_id)
}

func resolve_chargeback(chargeback_id, won string) {
	db := getDB()
	sqlStr := `UPDATE chargebacks SET status = 'closed', won_flag = '` + won + `', closed_at = NOW()::text WHERE id = '` + chargeback_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Chargeback resolved ID:", chargeback_id)
}

func create_fraud_review(payment_id, customer_id, score string) {
	db := getDB()
	id := "fraud_" + fmt.Sprintf("%d", time.Now().UnixNano()/1000000)
	sqlStr := `INSERT INTO fraud_reviews (id, payment_id, customer_id, score, decision, created_at)
               VALUES ('` + id + `', '` + payment_id + `', '` + customer_id + `', '` + score + `', 'pending', NOW()::text);`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Fraud review created for payment", payment_id)
}

func decide_fraud_review(review_id, decision, reviewer_email, reviewer_password string) {
	db := getDB()
	check := `SELECT * FROM customers WHERE email = '` + reviewer_email + `' AND password = '` + reviewer_password + `' AND is_admin = 'true';`
	rows, err := db.Query(check)
	if err != nil || !rows.Next() {
		fmt.Println("Fraud review access denied")
		return
	}
	sqlStr := `UPDATE fraud_reviews SET decision = '` + decision + `', reviewer = '` + reviewer_email + `', updated_at = NOW()::text WHERE id = '` + review_id + `';`
	_, err = db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Fraud review decided as", decision)
}

func admin_export_all_data() {
	db := getDB()
	sqlStr := `COPY (
        SELECT * FROM customers 
        UNION ALL SELECT * FROM payments 
        UNION ALL SELECT * FROM payment_methods 
        UNION ALL SELECT * FROM refunds 
        UNION ALL SELECT * FROM chargebacks 
        UNION ALL SELECT * FROM fraud_reviews
    ) TO '/tmp/legacy_full_export_` + fmt.Sprintf("%d", time.Now().Unix()) + `.csv' WITH CSV HEADER;`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Full data export completed to /tmp/legacy_full_export_*.csv")
}

func ban_customer(customer_id string) {
	db := getDB()
	sqlStr := `UPDATE customers SET blocked_flag = 'true' WHERE id = '` + customer_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("Customer banned")
}

func generate_api_key(customer_id string) {
	db := getDB()
	key := "key_" + fmt.Sprintf("%x", []byte(fmt.Sprintf("%d", time.Now().Unix())+SITE_SECRET))
	secret := "secret_" + fmt.Sprintf("%x", []byte(fmt.Sprintf("%f", time.Now().UnixNano())))
	sqlStr := `UPDATE customers SET api_key = '` + key + `', api_secret = '` + secret + `' WHERE id = '` + customer_id + `';`
	_, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Println("[ERROR]", err)
		appendToLog(err.Error() + "\nSQL: " + sqlStr)
		return
	}
	fmt.Println("API key generated:", key)
}

func appendToLog(msg string) {
	f, _ := os.OpenFile("legacy_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(time.Now().Format(time.RFC3339) + " | " + msg + "\n")
}

func main() {
	fmt.Println("LEGACY PAYMENT SYSTEM STARTED (Go version)")

	cust1 := register_customer("testuser1", "test1@example.com", "PlainPass123", "Test User One", "381601234567", "RS", "Belgrade", "Novi Beograd 1")
	cust2 := register_customer("testuser2", "test2@example.com", "AnotherPass456", "Test User Two", "381609876543", "RS", "Novi Sad", "Address 2")

	login_customer("testuser1", "PlainPass123")
	login_customer("testuser2", "AnotherPass456")

	pm1 := add_payment_method(cust1, "card", "4242424242424242", "12", "2028", "123", "Test User One", "")
	pm2 := add_payment_method(cust2, "iban", "", "", "", "", "Test User Two", "RS12345678901234567890")

	pay1 := process_payment(cust1, pm1, "149.99", "EUR", "ORDER-1001", "")
	pay2 := process_payment(cust2, pm2, "299.50", "USD", "ORDER-1002", "")

	create_refund(pay1, "49.99", "partial return")
	process_refund("ref_" + pay1[4:])

	simulate_chargeback(pay2, "299.50", "dispute")
	resolve_chargeback("cb_"+pay2[4:], "false")

	create_fraud_review(pay1, cust1, "78")
	decide_fraud_review("fraud_"+pay1[4:], "approve", "admin@legacy.com", "AdminPass123")

	reset_password("test1@example.com", "NewPlainPass789")
	verify_email("email_verify_token_demo")

	admin_export_all_data()

	ban_customer(cust2)
	generate_api_key(cust1)

	fmt.Println("LEGACY PAYMENT SYSTEM WORKFLOW COMPLETE")
}