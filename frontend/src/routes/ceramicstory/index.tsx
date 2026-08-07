import { createFileRoute } from "@tanstack/solid-router";
import { Tabs } from "@kobalte/core/tabs";
import { createSignal, For } from "solid-js";
import {
  CeramicStoryCard,
  type DynastyData,
} from "~/components/layout/CeramicstoryCard";
import "./index.css";

// Mock data that would normally come from the backend
const dynastyData: DynastyData[] = [
  {
    id: "song",
    slug: "song-dynasty",
    dynasty: "Song",
    period: "Northern & Southern",
    startYear: 960,
    endYear: 1279,
    description:
      "The Song dynasty was a culturally rich and sophisticated age for China. It saw the flourishing of many arts and sciences, and ceramics reached new heights of refinement. Wares were known for their simple, elegant forms and subtle, monochrome glazes, prized by the imperial court and scholar-officials.",
    takeaways: [
      "Development of famous kilns like Ru, Jun, and Ding.",
      "Emphasis on elegant, minimalist aesthetics.",
      "First use of true porcelain in some regions.",
    ],
    imageUrl: "https://placehold.co/400x600/d1e5f0/3a6a8a?text=Song",
  },
  {
    id: "yuan",
    slug: "yuan-dynasty",
    dynasty: "Yuan",
    period: "Mongol Empire",
    startYear: 1271,
    endYear: 1368,
    description:
      "The Yuan dynasty, established by Kublai Khan, saw a dramatic shift in ceramic production. The period is most famous for the development and perfection of underglaze blue-and-white porcelain, which became a major export commodity, influencing ceramic traditions across the globe.",
    takeaways: [
      "Perfection of cobalt blue underglaze decoration.",
      "Large-scale production for export markets.",
      "Fusion of Chinese, Mongol, and Middle Eastern motifs.",
    ],
    imageUrl: "https://placehold.co/400x600/f7f7f7/4d4d4d?text=Yuan",
  },
  {
    id: "ming",
    slug: "ming-dynasty",
    dynasty: "Ming",
    period: "Yongle & Xuande",
    startYear: 1368,
    endYear: 1644,
    description:
      "The Ming dynasty is often regarded as the golden age of Chinese porcelain. The imperial kilns at Jingdezhen produced an astonishing variety of high-quality wares, from delicate blue-and-white to vibrant polychrome enamels. Techniques like doucai and wucai were developed, showcasing incredible artistic skill.",
    takeaways: [
      "Imperial kilns at Jingdezhen dominated production.",
      "Introduction of colorful overglaze enamels (doucai, wucai).",
      "Blue-and-white porcelain reached peak artistic quality.",
    ],
    imageUrl: "https://placehold.co/400x600/fddbc7/8c510a?text=Ming",
  },
  {
    id: "qing",
    slug: "qing-dynasty",
    dynasty: "Qing",
    period: "Kangxi & Qianlong",
    startYear: 1644,
    endYear: 1912,
    description:
      "During the Qing dynasty, technical mastery of porcelain production reached its zenith. Potters achieved flawless white bodies and a dazzling array of glaze colors and decorative techniques. Famille rose and famille verte enamels became extremely popular, especially for export to Europe, where they were highly coveted.",
    takeaways: [
      "Unsurpassed technical perfection in porcelain.",
      "Development of famille rose and famille verte palettes.",
      "Massive export to Europe (Chinoiserie).",
    ],
    imageUrl: "https://placehold.co/400x600/c7eae5/01665e?text=Qing",
  },
];

export const Route = createFileRoute("/ceramicstory/")({
  component: CeramicstoryPage,
});

function CeramicstoryPage() {
  let tabsListEl: HTMLDivElement | undefined;
  const [selectedValue, setSelectedValue] = createSignal(dynastyData[0].id);

  const handleWheel = (e: WheelEvent) => {
    // Prevent the page from scrolling vertically
    e.preventDefault();

    const triggers = Array.from(
      tabsListEl?.querySelectorAll("[data-role='trigger']") || [],
    ) as HTMLElement[];
    if (triggers.length === 0) return;

    const currentIndex = triggers.findIndex(
      (trigger) => trigger.getAttribute("data-value") === selectedValue(),
    );
    if (currentIndex === -1) return;

    let nextIndex: number;

    if (e.deltaY > 0) {
      // Scrolling down -> move to the next tab (right)
      nextIndex = (currentIndex + 1) % triggers.length;
    } else {
      // Scrolling up -> move to the previous tab (left)
      nextIndex = (currentIndex - 1 + triggers.length) % triggers.length;
    }

    triggers[nextIndex]?.click();
  };

  return (
    <div class="w-full h-full flex flex-col p-8 bg-gray-50">
      <Tabs class="tabs" value={selectedValue()} onChange={setSelectedValue}>
        <Tabs.List ref={tabsListEl} class="tabs__list" onWheel={handleWheel}>
          <For each={dynastyData}>
            {(dynasty) => (
              <Tabs.Trigger class="tabs__trigger" value={dynasty.id}>
                {dynasty.dynasty}
              </Tabs.Trigger>
            )}
          </For>
        </Tabs.List>

        <div class="flex-grow mt-8">
          <For each={dynastyData}>
            {(dynasty) => (
              <Tabs.Content class="tabs__content" value={dynasty.id}>
                <CeramicStoryCard data={dynasty} />
              </Tabs.Content>
            )}
          </For>
        </div>
      </Tabs>
    </div>
  );
}
