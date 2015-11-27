/**
 * @fileoverview Description of this file.
 */
var main = function() {
  var API = SS.Api = {
    key: 'E*SVgMUlGvyMBrRucMfILA((',
    version: '2.2',
    sites: {},
    totalQuestions: 0,
    getQuestions: function(data, tags) {
      var SEurl = 'https://api.stackexchange.com/'+ Api.version + '/search?order=desc&sort=activity&tagged=google-places-api&site=stackoverflow';
      var ajaxOptions = {
        url: SEurl,
        dataType: 'json',
        type: 'GET'
      };
      return $.ajax(ajaxOptions)
          .then(function(resp) {
            var i;
            var len = resp.items.length;
            var arr = [];
            for (i = 0; i < len; i++) {
              console.log(resp.items[i]);
            }
          })
      ;
    }
  }
}



function init() {
   
}
