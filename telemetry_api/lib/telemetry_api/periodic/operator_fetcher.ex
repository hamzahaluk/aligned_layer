defmodule TelemetryApi.Periodic.OperatorFetcher do
  use GenServer
  alias TelemetryApi.Operators
  alias TelemetryApi.ContractManagers.RegistryCoordinatorManager
  require Logger

  @never_registered 0
  @registered 1
  @deregistered 2

  @wait_time_str System.get_env("OPERATOR_FETCHER_WAIT_TIME_MS") ||
    raise """
    environment variable OPERATOR_FETCHER_WAIT_TIME_MS is missing.
    """

  @wait_time_ms (
    case Integer.parse(@wait_time_str) do
      :error -> raise("OPERATOR_FETCHER_WAIT_TIME_MS is not a number, received: #{@wait_time_str}")
      {num, _} -> num
    end
  )

  def start_link(_) do
    GenServer.start_link(__MODULE__, %{})
  end

  def init(_) do
    send_work()
    {:ok, %{}}
  end

  def send_work() do
    :timer.send_interval(@wait_time_ms, :poll_service)
  end

  def handle_info(:poll_service, state) do
    fetch_operators_info()
    fetch_operators_status()
    {:noreply, state}
  end

  defp fetch_operators_info() do
    case Operators.fetch_all_operators() do
      {:ok, _} -> :ok
      {:error, message} -> IO.inspect("Couldn't fetch operators: #{IO.inspect(message)}")
    end
  end

  defp fetch_operators_status() do
    Operators.list_operators()
    |> Enum.map(fn op ->
      case RegistryCoordinatorManager.fetch_operator_status(op.address) do
        {:ok, status} ->
          Operators.update_operator(op, %{status: string_status(status)})

        error ->
          Logger.error("Error when updating status: #{error}")
      end
    end)
    :ok
  end

  defp string_status(@never_registered), do: "NEVER_REGISTERED"
  defp string_status(@registered), do: "REGISTERED"
  defp string_status(@deregistered), do: "DEREGISTERED"
end