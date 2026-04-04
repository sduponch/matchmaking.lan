// Map thumbnail paths — place images in frontend/public/maps/{mapname}.jpg
// CS2 official map names as keys.

export const MAP_DISPLAY_NAMES: Record<string, string> = {
  de_ancient:  'Ancient',
  de_anubis:   'Anubis',
  de_dust2:    'Dust II',
  de_inferno:  'Inferno',
  de_mirage:   'Mirage',
  de_nuke:     'Nuke',
  de_overpass: 'Overpass',
  de_train:    'Train',
  de_vertigo:  'Vertigo',
  cs_italy:    'Italy',
  cs_office:   'Office',
  ar_baggage:  'Baggage',
  ar_shoots:   'Shoots',
  dm_rust:     'Rust',
}

export function mapDisplayName(map: string): string {
  return MAP_DISPLAY_NAMES[map] ?? map
}

export function mapThumbnail(map: string): string {
  return `/maps/${map}.jpg`
}
