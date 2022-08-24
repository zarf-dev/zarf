<script lang="ts">
  import { GetZarfState } from "../wailsjs/go/ui/App";
  import logo from "./assets/images/zarf-logo.png";
</script>

<main>
  <img alt="Zarf logo" id="logo" src={logo} />

  <hr />

  {#await GetZarfState() then state}
  <pre>
    {JSON.stringify(state, null, 2)}
  </pre>
  {/await}
</main>

<style>
  #logo {
    display: block;
    margin: 1rem;
    background-repeat: no-repeat;
    background-size: 100% 100%;
  }

  pre {
    text-align: left;
  }
</style>
