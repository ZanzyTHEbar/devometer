import { createSignal } from "solid-js";
import { createPaymentSession, UserStats } from "../api";

interface PaymentModalProps {
  isOpen: boolean;
  onClose: () => void;
  userStats: UserStats;
  onPaymentSuccess: () => void;
}

export default function PaymentModal(props: PaymentModalProps) {
  const [selectedAmount, setSelectedAmount] = createSignal<number>(10);
  const [customAmount, setCustomAmount] = createSignal<string>("");
  const [isProcessing, setIsProcessing] = createSignal(false);
  const [paymentType, setPaymentType] = createSignal<"donation" | "unlimited">("donation");

  const predefinedAmounts = [5, 10, 25, 50, 100];

  const handlePayment = async () => {
    setIsProcessing(true);
    try {
      let amount: number | undefined;

      if (paymentType() === "donation") {
        if (customAmount()) {
          amount = parseFloat(customAmount());
          if (isNaN(amount) || amount <= 0) {
            throw new Error("Invalid custom amount");
          }
        } else {
          amount = selectedAmount();
        }
      }

      const session = await createPaymentSession(paymentType(), amount);
      window.location.href = session.url; // Redirect to Stripe Checkout
    } catch (error) {
      console.error("Payment error:", error);
      alert("Payment failed. Please try again.");
    } finally {
      setIsProcessing(false);
    }
  };

  if (!props.isOpen) return null;

  return (
    <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div class="bg-base-100 p-8 rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="flex justify-between items-center mb-6">
          <h2 class="text-2xl font-bold text-base-content">
            {paymentType() === "unlimited" ? "Unlimited Access" : "Support the Project"}
          </h2>
          <button onClick={props.onClose} class="btn btn-sm btn-ghost">
            ✕
          </button>
        </div>

        {/* Payment Type Toggle */}
        <div class="flex gap-2 mb-6">
          <button
            onClick={() => setPaymentType("donation")}
            class={`btn flex-1 ${paymentType() === "donation" ? "btn-primary" : "btn-outline"}`}
          >
            💝 Donate
          </button>
          <button
            onClick={() => setPaymentType("unlimited")}
            class={`btn flex-1 ${paymentType() === "unlimited" ? "btn-primary" : "btn-outline"}`}
          >
            🚀 Unlimited Access
          </button>
        </div>

        {/* Usage Info */}
        <div class="bg-base-200 p-4 rounded-lg mb-6">
          <h3 class="font-semibold mb-2">Your Current Usage</h3>
          <div class="space-y-1 text-sm">
            <p>Requests this week: {props.userStats.requests_this_week}/5</p>
            <p>
              Remaining:{" "}
              {props.userStats.remaining_requests === -1
                ? "Unlimited"
                : props.userStats.remaining_requests}
            </p>
            <p>Status: {props.userStats.is_paid ? "Premium User" : "Free Tier"}</p>
          </div>
        </div>

        {paymentType() === "donation" ? (
          <>
            {/* Predefined Amounts */}
            <div class="mb-4">
              <h3 class="font-semibold mb-3">Select Amount</h3>
              <div class="grid grid-cols-3 gap-2 mb-4">
                {predefinedAmounts.map((amount) => (
                  <button
                    onClick={() => {
                      setSelectedAmount(amount);
                      setCustomAmount("");
                    }}
                    class={`btn ${selectedAmount() === amount && !customAmount() ? "btn-primary" : "btn-outline"}`}
                  >
                    ${amount}
                  </button>
                ))}
              </div>
            </div>

            {/* Custom Amount */}
            <div class="mb-6">
              <label class="label">
                <span class="label-text">Or enter custom amount</span>
              </label>
              <input
                type="number"
                placeholder="Enter amount in USD"
                value={customAmount()}
                onInput={(e) => {
                  setCustomAmount(e.currentTarget.value);
                  if (e.currentTarget.value) {
                    setSelectedAmount(0);
                  }
                }}
                class="input input-bordered w-full"
                min="1"
                step="0.01"
              />
            </div>
          </>
        ) : (
          <div class="bg-success bg-opacity-20 p-4 rounded-lg mb-6">
            <h3 class="font-semibold text-success mb-2">🎉 Unlimited Access</h3>
            <ul class="text-sm space-y-1">
              <li>• Unlimited analysis requests</li>
              <li>• Priority support</li>
              <li>• Early access to new features</li>
              <li>• Monthly subscription: $9.99</li>
            </ul>
          </div>
        )}

        {/* Action Buttons */}
        <div class="flex gap-3">
          <button onClick={props.onClose} class="btn btn-outline flex-1" disabled={isProcessing()}>
            Cancel
          </button>
          <button onClick={handlePayment} class="btn btn-primary flex-1" disabled={isProcessing()}>
            {isProcessing() ? (
              <span class="loading loading-spinner loading-sm"></span>
            ) : paymentType() === "unlimited" ? (
              "Subscribe Now"
            ) : (
              `Donate $${customAmount() || selectedAmount()}`
            )}
          </button>
        </div>

        <p class="text-xs text-base-content/70 mt-4 text-center">
          Secure payment powered by Stripe
        </p>
      </div>
    </div>
  );
}
