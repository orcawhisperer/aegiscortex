import Studio from "../components/Studio";
import { loadStudioBoot } from "../lib/backend";

export const dynamic = "force-dynamic";

export default async function Page() {
  const boot = await loadStudioBoot();
  return <Studio boot={boot} />;
}
