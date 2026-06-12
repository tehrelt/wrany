import { defineConfig } from 'orval'

// Used by `make ts-client` for offline codegen from local swagger.json.
// For live server codegen use: npm run gen:api
export default defineConfig({
  wranyApi: {
    input: {
      target: process.env.ORVAL_INPUT ?? '../../services/tracking-gateway/docs/swagger.json',
    },
    output: {
      mode: 'tags-split',
      target: 'src/api/generated',
      schemas: 'src/api/generated/model',
      client: 'axios',
      clean: true,
      override: {
        mutator: {
          path: 'src/api/client.ts',
          name: 'customRequest',
        },
        components: {
          schemas: {
            suffix: '',
          },
        },
      },
    },
  },
})
